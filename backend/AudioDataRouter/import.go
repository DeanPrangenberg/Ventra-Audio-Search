package AudioDataRouter

import (
	"encoding/csv"
	"fmt"
	"go_audio_search_api_server/ai"
	"go_audio_search_api_server/globalTypes"
	"go_audio_search_api_server/globalUtils"
	"log/slog"
	"strconv"
	"strings"
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
	case int(globalTypes.StageSummary):
		return "summary"
	default:
		return "unknown_stage_" + strconv.Itoa(stage)
	}
}

func logImport(level slog.Level, msg string, workerIdx int, audioDataElement globalTypes.AudioDataElement, extra ...any) {
	attrs := []any{
		"worker", workerIdx,
		"audioHash", audioDataElement.AudiofileHash,
		"stage", stageName(int(audioDataElement.LastSuccessfulStep)),
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

func (w *RoutWorker) StartImportAudioDataWorker(idx int) {
	slog.Debug("import worker started", "worker", idx)

	for {
		select {
		case <-w.StopCtx.Done():
			slog.Debug("import worker stopped", "worker", idx, "reason", "stop_ctx_done")
			return

		case audioDataElement := <-w.AudioDataProcessingJobs:
			if err := w.HandleImportAudioDataJob(idx, audioDataElement); err != nil {
				slog.Error(
					"import job failed",
					"worker", idx,
					"audioHash", audioDataElement.AudiofileHash,
					"stage", stageName(int(audioDataElement.LastSuccessfulStep)),
					"retry", audioDataElement.RetryCounter,
					"err", err,
				)
			}
		}
	}
}

func (w *RoutWorker) HandleImportAudioDataJob(workerIdx int, audioDataElement globalTypes.AudioDataElement) error {
	logImport(slog.LevelInfo, "import job started", workerIdx, audioDataElement)

	if audioDataElement.LastSuccessfulStep == globalTypes.StageReceived {
		logImport(slog.LevelDebug, "persisting file to disk", workerIdx, audioDataElement)

		err, updatedElement := saveAudiofileElementToDisk(&audioDataElement)
		if err != nil {
			return w.updateRetryCounter(workerIdx, &audioDataElement, err)
		}

		ctx, cancel := w.opCtx()
		w.dbLock.Lock()
		err = w.db.UpdateAudiofileHash(ctx, audioDataElement.AudiofileHash, updatedElement.AudiofileHash)
		w.dbLock.Unlock()
		cancel()
		if err != nil {
			return err
		}

		logImport(
			slog.LevelDebug,
			"file persisted",
			workerIdx,
			*updatedElement,
			"newAudioHash", updatedElement.AudiofileHash,
		)

		err = w.createNewState(updatedElement)
		if err != nil {
			return err
		}
	}

	if audioDataElement.LastSuccessfulStep == globalTypes.StageFilePersisted {
		logImport(slog.LevelDebug, "starting transcription", workerIdx, audioDataElement)

		w.whisperLock.Lock()
		ctx, cancel := w.opCtx()
		result, err := w.whisper.Transcribe(ctx, audioDataElement.DownloadPath)
		w.whisperLock.Unlock()
		cancel()
		if err != nil {
			return w.updateRetryCounter(workerIdx, &audioDataElement, err)
		}

		audioDataElement.TranscriptFull = result.Transcript

		for _, segment := range result.Segments {
			hashInput := fmt.Sprintf("%s-%f-%f-%s", audioDataElement.AudiofileHash, segment.Start, segment.End, segment.Transcript)
			builtSegment := globalTypes.SegmentElement{
				AudiofileHash: audioDataElement.AudiofileHash,
				StartInSec:    segment.Start,
				EndInSec:      segment.End,
				Transcript:    segment.Transcript,
				SegmentHash:   globalUtils.StringSha256Hex(hashInput),
			}

			*audioDataElement.SegmentElements = append(*audioDataElement.SegmentElements, builtSegment)
		}

		logImport(
			slog.LevelDebug,
			"transcription finished",
			workerIdx,
			audioDataElement,
			"transcriptLen", len(result.Transcript),
			"segmentCount", len(result.Segments),
		)

		w.dbLock.Lock()
		ctx, cancel = w.opCtx()
		err = w.db.UpdateTranscriptFull(ctx, audioDataElement.AudiofileHash, audioDataElement.TranscriptFull)
		w.dbLock.Unlock()
		cancel()
		if err != nil {
			return w.updateRetryCounter(workerIdx, &audioDataElement, err)
		}

		w.dbLock.Lock()
		ctx, cancel = w.opCtx()
		err = w.db.InsertSegmentsUpsert(ctx, audioDataElement.AudiofileHash, audioDataElement.SegmentElements)
		w.dbLock.Unlock()
		cancel()
		if err != nil {
			return w.updateRetryCounter(workerIdx, &audioDataElement, err)
		}

		logImport(
			slog.LevelDebug,
			"transcript and segments stored",
			workerIdx,
			audioDataElement,
			"segmentCount", len(result.Segments),
		)

		err = w.createNewState(&audioDataElement)
		if err != nil {
			return err
		}
	}

	if audioDataElement.LastSuccessfulStep == globalTypes.StageTranscript {
		logImport(slog.LevelDebug, "creating segment embeddings", workerIdx, audioDataElement)

		var segments []globalTypes.SegmentElement

		var err error
		w.dbLock.Lock()
		ctx, cancel := w.opCtx()
		audioDataElement.SegmentElements, err = w.db.GetAllSegmentsByAudioHash(ctx, audioDataElement.AudiofileHash)
		w.dbLock.Unlock()
		cancel()
		if err != nil {
			return w.updateRetryCounter(workerIdx, &audioDataElement, err)
		}

		for _, segment := range *audioDataElement.SegmentElements {
			w.embeddingsLock.Lock()
			embedding, err := w.embeddings.CreateEmbedding(segment.Transcript)
			w.embeddingsLock.Unlock()
			if err != nil {
				return w.updateRetryCounter(workerIdx, &audioDataElement, err)
			}

			segment.TranscriptEmbedding = embedding
			segments = append(segments, segment)
		}

		logImport(
			slog.LevelDebug,
			"segment embeddings created",
			workerIdx,
			audioDataElement,
			"segmentCount", len(segments),
		)

		w.qdrantLock.Lock()
		ctx, cancel = w.opCtx()
		err = w.qdrant.UpsertSegmentEmbedding(ctx, segments)
		w.qdrantLock.Unlock()
		cancel()
		if err != nil {
			return w.updateRetryCounter(workerIdx, &audioDataElement, err)
		}

		logImport(
			slog.LevelDebug,
			"segment embeddings stored in vector db",
			workerIdx,
			audioDataElement,
			"segmentCount", len(segments),
		)

		err = w.createNewState(&audioDataElement)
		if err != nil {
			return err
		}
	}

	if audioDataElement.LastSuccessfulStep == globalTypes.StageEmbeddings {
		logImport(slog.LevelDebug, "generating ai summary", workerIdx, audioDataElement)

		_, summarySysPrompt := ai.GetSysPrompts(audioDataElement.AudiofileHash)

		w.llmLock.Lock()
		summary, err := ai.OllamaRequest(
			globalUtils.LoadEnv("LLM_MODEL"),
			globalUtils.LoadEnv("OLLAMA_API_URL")+"/api/chat",
			summarySysPrompt,
			audioDataElement.TranscriptFull,
		)
		w.llmLock.Unlock()
		if err != nil {
			return w.updateRetryCounter(workerIdx, &audioDataElement, err)
		}

		audioDataElement.AiSummary = summary

		w.dbLock.Lock()
		ctx, cancel := w.opCtx()
		err = w.db.UpdateAISummary(ctx, audioDataElement.AudiofileHash, audioDataElement.AiSummary)
		w.dbLock.Unlock()
		cancel()
		if err != nil {
			return w.updateRetryCounter(workerIdx, &audioDataElement, err)
		}

		logImport(
			slog.LevelDebug,
			"ai summary stored",
			workerIdx,
			audioDataElement,
			"summaryLen", len(summary),
		)

		err = w.createNewState(&audioDataElement)
		if err != nil {
			return err
		}
	}

	if audioDataElement.LastSuccessfulStep == globalTypes.StageSummary {
		logImport(slog.LevelDebug, "generating ai keywords", workerIdx, audioDataElement)

		keywordSysPrompt, _ := ai.GetSysPrompts(audioDataElement.AudiofileHash)

		w.llmLock.Lock()
		keywords, err := ai.OllamaRequest(
			globalUtils.LoadEnv("LLM_MODEL"),
			globalUtils.LoadEnv("OLLAMA_API_URL")+"/api/chat",
			keywordSysPrompt,
			audioDataElement.TranscriptFull,
		)
		w.llmLock.Unlock()
		if err != nil {
			return w.updateRetryCounter(workerIdx, &audioDataElement, err)
		}

		r := csv.NewReader(strings.NewReader(keywords))
		record, err := r.Read()
		if err != nil {
			return w.updateRetryCounter(workerIdx, &audioDataElement, err)
		}

		for i := range record {
			record[i] = strings.TrimSpace(record[i])
		}

		audioDataElement.AiKeywords = record

		w.dbLock.Lock()
		ctx, cancel := w.opCtx()
		err = w.db.UpdateAIKeywords(ctx, audioDataElement.AudiofileHash, audioDataElement.AiKeywords)
		w.dbLock.Unlock()
		cancel()
		if err != nil {
			return w.updateRetryCounter(workerIdx, &audioDataElement, err)
		}

		logImport(
			slog.LevelDebug,
			"ai keywords stored",
			workerIdx,
			audioDataElement,
			"keywordCount", len(record),
		)

		err = w.createNewState(&audioDataElement)
		if err != nil {
			return err
		}
	}

	logImport(slog.LevelInfo, "import job finished", workerIdx, audioDataElement)
	return nil
}

func (w *RoutWorker) createNewState(audioDataElement *globalTypes.AudioDataElement) error {
	audioDataElement.UpdateToNextStage()

	w.dbLock.Lock()
	ctx, cancel := w.opCtx()
	err := w.db.UpsertBase(ctx, audioDataElement)
	w.dbLock.Unlock()
	cancel()

	return err
}

func (w *RoutWorker) updateRetryCounter(workerIdx int, audioDataElement *globalTypes.AudioDataElement, cause error) error {
	audioDataElement.RetryCounter++

	logImport(
		slog.LevelWarn,
		"stage failed, incremented retry counter",
		workerIdx,
		*audioDataElement,
		"err", cause,
	)

	w.dbLock.Lock()
	ctx, cancel := w.opCtx()
	err := w.db.UpsertBase(ctx, audioDataElement)
	w.dbLock.Unlock()
	cancel()

	if err != nil {
		return fmt.Errorf("original error: %w; additionally failed to persist retry counter: %v", cause, err)
	}

	return cause
}
