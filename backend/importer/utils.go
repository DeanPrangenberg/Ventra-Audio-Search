package importer

import (
	"go_audio_search_api_server/globalTypes"
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

func logImport(level slog.Level, msg string, workerIdx int, audioDataElement globalTypes.AudioDataElement, extra ...any) {
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
