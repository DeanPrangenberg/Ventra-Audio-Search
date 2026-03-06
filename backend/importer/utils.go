package importer

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"go_audio_search_api_server/globalTypes"
	"go_audio_search_api_server/globalUtils"
	"log/slog"
	"strconv"
)

func stageName(stage int) string {
	switch stage {
	case int(globalTypes.StageReceived):
		return "received"
	case int(globalTypes.StageFilePersisted):
		return "file_persisted"
	case int(globalTypes.StageTranscript):
		return "transcript"
	case int(globalTypes.StageEmbeddings):
		return "embeddings"
	case int(globalTypes.StageAiGeneration):
		return "ai_generation"
	default:
		return "unknown_stage_" + strconv.Itoa(stage)
	}
}

func logImport(level slog.Level, msg string, workerIdx uint, audioDataElement *globalTypes.AudioDataElement, extra ...any) {
	attrs := []any{
		"worker", workerIdx,
		"audioHash", audioDataElement.AudiofileHash,
		"stage", stageName(int(audioDataElement.LastSuccessfulStage)),
		"retry", audioDataElement.RetryCounter,
	}
	attrs = append(attrs, extra...)

	switch {
	case level <= slog.LevelDebug:
		slog.Debug(msg, attrs...)
	case level <= slog.LevelInfo:
		slog.Info(msg, attrs...)
	case level <= slog.LevelWarn:
		slog.Warn(msg, attrs...)
	default:
		slog.Error(msg, attrs...)
	}
}

func saveAudiofileElementToDisk(ctx context.Context, element *globalTypes.AudioDataElement) (error, *globalTypes.AudioDataElement) {
	hasURL := element.FileUrl != ""
	hasB64 := element.Base64Data != ""

	if !hasURL && !hasB64 {
		return errors.New("tried to save a file without FileUrl or Base64Data"), nil
	}

	initSeed := element.FileUrl
	if initSeed == "" {
		initSeed = element.Base64Data
	}
	initName := globalUtils.StringSha256Hex(initSeed)

	switch {
	case hasURL:
		slog.Info("downloading from url", "url", element.FileUrl)

		err, new_path := globalUtils.DownloadURLToFile(ctx, element.FileUrl, initName)
		if err != nil {
			return fmt.Errorf("error while downloading '%s': %w", element.FileUrl, err), nil
		}

		path, hash, err := globalUtils.MarkFileAtomicMP3(new_path)
		if err != nil {
			return fmt.Errorf("error while marking file as mp3 '%s': %w", initName, err), nil
		}

		element.DownloadPath = path
		element.AudiofileHash = hash
		element.FileUrl = ""
		element.Base64Data = ""

	case hasB64:
		slog.Info("writing from base64")

		decodedBytes, err := base64.StdEncoding.DecodeString(element.Base64Data)
		if err != nil {
			return fmt.Errorf("base64 decode failed: %w", err), nil
		}

		filePath, err := globalUtils.WriteFileAtomicMP3(decodedBytes, initName)
		if err != nil {
			return fmt.Errorf("error while writing file '%s': %w", initName, err), nil
		}

		path, hash, err := globalUtils.MarkFileAtomicMP3(filePath)
		if err != nil {
			return fmt.Errorf("error while marking file as mp3 '%s': %w", initName, err), nil
		}

		element.DownloadPath = path
		element.AudiofileHash = hash
		element.Base64Data = ""
		element.FileUrl = ""
	}

	return nil, element
}

func notify(wake chan struct{}) {
	slog.Debug("Notifying Dispatcher about new audio data")
	select {
	case wake <- struct{}{}:
	default:
	}
}

func (w *Worker) opCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(w.StopCtx, opTimeout)
}

func (w *Worker) updateStage(audioDataElement *globalTypes.AudioDataElement) error {
	audioDataElement.UpdateToNextStage()

	ctx, cancel := w.opCtx()
	err := w.postgres.UpsertBase(ctx, audioDataElement)
	cancel()

	return err
}

func (w *Worker) updateRetryCounter(workerIdx uint, audioDataElement *globalTypes.AudioDataElement, cause error) error {
	audioDataElement.RetryCounter++

	logImport(
		slog.LevelWarn,
		"stage failed, incremented retry counter",
		workerIdx,
		audioDataElement,
		"err", cause,
	)

	ctx, cancel := w.opCtx()
	err := w.postgres.UpsertBase(ctx, audioDataElement)
	cancel()

	if err != nil {
		return fmt.Errorf("original error: %w; additionally failed to persist retry counter: %v", cause, err)
	}

	return cause
}
