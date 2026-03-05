package importer

import (
	"fmt"
	"go_audio_search_api_server/FlowManager"
	"go_audio_search_api_server/globalTypes"
	"go_audio_search_api_server/globalUtils"
	"log/slog"
)

// persistFile saves the audio file to disk and updates the database with the new file path and hash.
func (w *FlowManager.FlowWorker) persistFile(workerIdx int, audioDataElement globalTypes.AudioDataElement) error {
	FlowManager.logImport(slog.LevelDebug, "persisting file to disk", workerIdx, audioDataElement)
	ctx, cancel := w.opCtx()
	err, updatedElement := FlowManager.saveAudiofileElementToDisk(ctx, &audioDataElement)
	cancel()

	if err != nil {
		return w.updateRetryCounter(workerIdx, &audioDataElement, err)
	}

	ctx, cancel = w.opCtx()
	err = w.db.UpdateAudiofileHash(ctx, audioDataElement.AudiofileHash, updatedElement.AudiofileHash)
	cancel()

	if err != nil {
		return err
	}

	FlowManager.logImport(
		slog.LevelDebug,
		"file persisted",
		workerIdx,
		*updatedElement,
		"newAudioHash", updatedElement.AudiofileHash,
	)

	err = w.updateStage(updatedElement)
	if err != nil {
		return err
	}

	return nil
}

// transcribeAudio transcribes the given audio data element, creates the segments and stores them in the database.
func (w *FlowManager.FlowWorker) transcribeAudio(workerIdx int, audioDataElement globalTypes.AudioDataElement) error {
	FlowManager.logImport(slog.LevelDebug, "starting transcription", workerIdx, audioDataElement)

	ctx, cancel := w.opCtx()
	result, err := w.whisper.Transcribe(ctx, audioDataElement.DownloadPath)
	cancel()

	if err != nil {
		return w.updateRetryCounter(workerIdx, &audioDataElement, err)
	}

	audioDataElement.TranscriptFull = result.Transcript
	audioDataElement.SegmentElements = []globalTypes.SegmentElement{}

	for _, segment := range result.Segments {
		hashInput := fmt.Sprintf("%s-%f-%f-%s", audioDataElement.AudiofileHash, segment.Start, segment.End, segment.Transcript)

		builtSegment := globalTypes.SegmentElement{
			AudiofileHash: audioDataElement.AudiofileHash,
			StartInSec:    segment.Start,
			EndInSec:      segment.End,
			Transcript:    segment.Transcript,
			SegmentHash:   globalUtils.StringSha256Hex(hashInput),
		}

		audioDataElement.SegmentElements = append(audioDataElement.SegmentElements, builtSegment)
	}

	FlowManager.logImport(
		slog.LevelDebug,
		"transcription finished",
		workerIdx,
		audioDataElement,
		"transcriptLen", len(result.Transcript),
		"segmentCount", len(result.Segments),
	)

	err = w.updateStage(&audioDataElement)
	if err != nil {
		return err
	}

	return nil
}

// createEmbeddings creates the embeddings for the segments of the given audio data element and stores them in the vector database.
func (w *FlowManager.FlowWorker) createEmbeddings(workerIdx int, audioDataElement globalTypes.AudioDataElement) error {
	FlowManager.logImport(slog.LevelDebug, "creating segment embeddings", workerIdx, audioDataElement)

	var segments []globalTypes.SegmentElement

	var err error
	ctx, cancel := w.opCtx()
	audioDataElement.SegmentElements, err = w.db.GetAllSegmentsByAudioHash(ctx, audioDataElement.AudiofileHash)
	cancel()

	if err != nil {
		return w.updateRetryCounter(workerIdx, &audioDataElement, err)
	}

	for _, segment := range audioDataElement.SegmentElements {
		embedding, err := w.embeddings.CreateEmbedding(segment.Transcript)
		if err != nil {
			return w.updateRetryCounter(workerIdx, &audioDataElement, err)
		}

		segment.TranscriptEmbedding = embedding
		segments = append(segments, segment)
	}

	FlowManager.logImport(
		slog.LevelDebug,
		"segment embeddings created",
		workerIdx,
		audioDataElement,
		"segmentCount", len(segments),
	)

	ctx, cancel = w.opCtx()
	err = w.qdrant.UpsertSegmentEmbeddings(ctx, segments)

	cancel()
	if err != nil {
		return w.updateRetryCounter(workerIdx, &audioDataElement, err)
	}

	FlowManager.logImport(
		slog.LevelDebug,
		"segment embeddings stored in vector db",
		workerIdx,
		audioDataElement,
		"segmentCount", len(segments),
	)

	err = w.updateStage(&audioDataElement)
	if err != nil {
		return err
	}

	return nil
}

// generateAiData creates the ai summary and keywords for the given audio data element and stores them in the database.
func (w *FlowManager.FlowWorker) generateAiData(workerIdx int, audioDataElement globalTypes.AudioDataElement) error {
	var err error

	audioDataElement.AiSummary, err = w.llm.Summary(audioDataElement.AudioType, audioDataElement.TranscriptFull)

	if err != nil {
		return w.updateRetryCounter(workerIdx, &audioDataElement, err)
	}

	FlowManager.logImport(
		slog.LevelDebug,
		"Created ai summary",
		workerIdx,
		audioDataElement,
		"summaryLen", len(audioDataElement.AiSummary),
	)

	audioDataElement.AiKeywords, err = w.llm.Keywords(audioDataElement.AudioType, audioDataElement.TranscriptFull)

	if err != nil {
		return w.updateRetryCounter(workerIdx, &audioDataElement, err)
	}

	FlowManager.logImport(
		slog.LevelDebug,
		"ai keywords stored",
		workerIdx,
		audioDataElement,
		"keywordCount", len(audioDataElement.AiKeywords),
	)

	err = w.updateStage(&audioDataElement)
	if err != nil {
		return err
	}

	return nil
}
