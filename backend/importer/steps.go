package importer

import (
    "fmt"
    "go_audio_search_api_server/globalTypes"
    "go_audio_search_api_server/globalUtils"
    "log/slog"
)

// persistFile saves the audio file to disk and updates the database with the new file path and hash.
func (w *Worker) persistFile(workerIdx uint, audioDataElement *globalTypes.AudioDataElement) error {
    logImport(slog.LevelDebug, "persisting file to disk", workerIdx, audioDataElement)

    oldHash := audioDataElement.AudiofileHash

    ctx, cancel := w.opCtx()
    err, updatedElement := saveAudiofileElementToDisk(ctx, audioDataElement)
    cancel()
    if err != nil {
        return w.updateRetryCounter(workerIdx, audioDataElement, err)
    }

    newHash := updatedElement.AudiofileHash

    ctx, cancel = w.opCtx()
    err = w.postgres.UpdateAudiofileHash(ctx, oldHash, newHash)
    cancel()
    if err != nil {
        return err
    }

    logImport(
        slog.LevelDebug,
        "file persisted",
        workerIdx,
        updatedElement,
        "oldAudioHash", oldHash,
        "newAudioHash", newHash,
    )

    err = w.updateStage(updatedElement)
    if err != nil {
        return err
    }

    return nil
}

// transcribeAudio transcribes the given audio data element, creates the segments and stores them in the database.
func (w *Worker) transcribeAudio(workerIdx uint, audioDataElement *globalTypes.AudioDataElement) error {
    logImport(slog.LevelDebug, "starting transcription", workerIdx, audioDataElement)

    ctx, cancel := w.opCtx()
    result, err := w.whisper.Transcribe(ctx, audioDataElement.DownloadPath)
    cancel()

    if err != nil {
        return w.updateRetryCounter(workerIdx, audioDataElement, err)
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

    logImport(
        slog.LevelDebug,
        "transcription finished",
        workerIdx,
        audioDataElement,
        "transcriptLen", len(result.Transcript),
        "segmentCount", len(result.Segments),
    )

    err = w.postgres.UpsertSegments(ctx, audioDataElement.AudiofileHash, &audioDataElement.SegmentElements)
    if err != nil {
        return w.updateRetryCounter(workerIdx, audioDataElement, err)
    }

    err = w.updateStage(audioDataElement)
    if err != nil {
        return err
    }

    return nil
}

// createEmbeddings creates the embeddings for the segments of the given audio data element and stores them in the vector database.
func (w *Worker) createEmbeddings(workerIdx uint, audioDataElement *globalTypes.AudioDataElement) error {
    logImport(slog.LevelDebug, "creating segment embeddings", workerIdx, audioDataElement)

    var segments []globalTypes.SegmentElement

    var err error
    ctx, cancel := w.opCtx()
    audioDataElement.SegmentElements, err = w.postgres.GetAllSegmentsByAudioHash(ctx, audioDataElement.AudiofileHash)
    cancel()

    if err != nil {
        return w.updateRetryCounter(workerIdx, audioDataElement, err)
    }

    if len(audioDataElement.SegmentElements) == 0 {
        logImport(
            slog.LevelWarn,
            "no segments found for audio element, skipping embedding creation",
            workerIdx,
            audioDataElement,
        )
    }

    for _, segment := range audioDataElement.SegmentElements {
        embedding, err := w.embeddings.CreateEmbedding(segment.Transcript)
        if err != nil {
            return w.updateRetryCounter(workerIdx, audioDataElement, err)
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

    ctx, cancel = w.opCtx()
    err = w.qdrant.UpsertSegmentEmbeddings(ctx, &segments)

    cancel()
    if err != nil {
        return w.updateRetryCounter(workerIdx, audioDataElement, err)
    }

    logImport(
        slog.LevelDebug,
        "segment embeddings stored in vector postgres",
        workerIdx,
        audioDataElement,
        "segmentCount", len(segments),
    )

    err = w.updateStage(audioDataElement)
    if err != nil {
        return err
    }

    return nil
}

// generateAiData creates the ai summary and keywords for the given audio data element and stores them in the database.
func (w *Worker) generateAiData(workerIdx uint, audioDataElement *globalTypes.AudioDataElement) error {
    var err error

    audioDataElement.AiSummary, err = w.llm.Summary(audioDataElement.AudioType, audioDataElement.TranscriptFull)

    if err != nil {
        return w.updateRetryCounter(workerIdx, audioDataElement, err)
    }

    logImport(
        slog.LevelDebug,
        "Created ai summary",
        workerIdx,
        audioDataElement,
        "summaryLen", len(audioDataElement.AiSummary),
    )

    audioDataElement.AiKeywords, err = w.llm.Keywords(audioDataElement.AudioType, audioDataElement.TranscriptFull)

    if err != nil {
        return w.updateRetryCounter(workerIdx, audioDataElement, err)
    }

    logImport(
        slog.LevelDebug,
        "ai keywords stored",
        workerIdx,
        audioDataElement,
        "keywordCount", len(audioDataElement.AiKeywords),
    )

    err = w.updateStage(audioDataElement)
    if err != nil {
        return err
    }

    return nil
}
