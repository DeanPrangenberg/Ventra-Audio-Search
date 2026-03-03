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

func (w *RoutWorker) StartImportAudioDataWorker(audioDataElement globalTypes.AudioDataElement) error {
	for {
		select {
		case <-w.StopCtx.Done():
			return nil

		case <-w.NewAudioDataSignal:
			err := w.HandleImportAudioDataJob(audioDataElement)
			if err != nil {
				slog.Error("Error handling import audio data job: " + err.Error())
			}

			return err
		}
	}
}

func (w *RoutWorker) HandleImportAudioDataJob(audioDataElement globalTypes.AudioDataElement) error {
	slog.Info("Starting import job at stage " + strconv.Itoa(int(audioDataElement.LastSuccessfulStep)) + " for file: " + audioDataElement.AudiofileHash)

	if audioDataElement.LastSuccessfulStep == globalTypes.StageReceived {
		err, updatedElement := saveAudiofileElementToDisk(&audioDataElement)
		if err != nil {
			err := w.updateRetryCounter(&audioDataElement)
			if err != nil {
				return err
			}

			return err
		}

		slog.Debug("Created audio file with hash: " + updatedElement.AudiofileHash)

		w.dbLock.Lock()
		err = w.db.UpdateAudiofileHash(w.TimeoutCtx, audioDataElement.AudiofileHash, updatedElement.AudiofileHash)
		w.dbLock.Unlock()

		if err != nil {
			return err
		}

		err = w.createNewState(updatedElement)
		if err != nil {
			return err
		}
	}

	if audioDataElement.LastSuccessfulStep == globalTypes.StageFilePersisted {
		slog.Info("Creating Transcript for file: " + audioDataElement.AudiofileHash)

		w.whisperLock.Lock()
		result, err := w.whisper.Transcribe(w.TimeoutCtx, audioDataElement.DownloadPath)
		w.whisperLock.Unlock()
		if err != nil {
			err := w.updateRetryCounter(&audioDataElement)
			if err != nil {
				return err
			}

			return err
		}

		slog.Debug(fmt.Sprintf("Transcript for file is done (Transcript Len: %d): %s", len(result.Transcript), audioDataElement.AudiofileHash))

		audioDataElement.TranscriptFull = result.Transcript

		slog.Debug(fmt.Sprintf("Creating Segments for file: %s", audioDataElement.AudiofileHash))

		for idx, segment := range result.Segments {
			hashInput := fmt.Sprintf("%s-%f-%f-%s", audioDataElement.AudiofileHash, segment.Start, segment.End, segment.Transcript)
			builtSegment := globalTypes.SegmentElement{
				AudiofileHash: audioDataElement.AudiofileHash,
				StartInSec:    segment.Start,
				EndInSec:      segment.End,
				Transcript:    segment.Transcript,
				SegmentHash:   globalUtils.StringSha256Hex(hashInput),
			}

			slog.Debug(fmt.Sprintf("Created Segment %d/%d (Len in sec: %.2f): %s", idx, len(result.Segments), segment.End-segment.Start, audioDataElement.AudiofileHash))

			*audioDataElement.SegmentElements = append(*audioDataElement.SegmentElements, builtSegment)
		}

		w.dbLock.Lock()
		err = w.db.UpdateTranscriptFull(w.TimeoutCtx, audioDataElement.AudiofileHash, audioDataElement.TranscriptFull)
		w.dbLock.Unlock()
		if err != nil {
			err := w.updateRetryCounter(&audioDataElement)
			if err != nil {
				return err
			}

			return err
		}

		slog.Debug("Inserted transcript into DB for file: " + audioDataElement.AudiofileHash)

		w.dbLock.Lock()
		err = w.db.InsertSegmentsUpsert(w.TimeoutCtx, audioDataElement.AudiofileHash, audioDataElement.SegmentElements)
		w.dbLock.Unlock()
		if err != nil {
			err := w.updateRetryCounter(&audioDataElement)
			if err != nil {
				return err
			}

			return err
		}

		err = w.createNewState(&audioDataElement)
		if err != nil {
			return err
		}
	}

	if audioDataElement.LastSuccessfulStep == globalTypes.StageTranscript {
		slog.Debug("Creating Embeddings for Segments for file: " + audioDataElement.AudiofileHash)

		var segments []globalTypes.SegmentElement

		var err error
		w.dbLock.Lock()
		audioDataElement.SegmentElements, err = w.db.GetAllSegmentsByAudioHash(w.TimeoutCtx, audioDataElement.AudiofileHash)
		w.dbLock.Unlock()
		if err != nil {
			err := w.updateRetryCounter(&audioDataElement)
			if err != nil {
				return err
			}

			return err
		}

		for _, segment := range *audioDataElement.SegmentElements {
			w.embeddingsLock.Lock()
			embedding, err := w.embeddings.CreateEmbedding(segment.Transcript)
			w.embeddingsLock.Unlock()
			if err != nil {
				err := w.updateRetryCounter(&audioDataElement)
				if err != nil {
					return err
				}

				return err
			}

			segment.TranscriptEmbedding = embedding
			segments = append(segments, segment)
		}

		slog.Debug(strconv.FormatInt(int64(len(segments)), 10) + " embeddings segments created for file: " + audioDataElement.AudiofileHash)

		w.qdrantLock.Lock()
		err = w.qdrant.UpsertSegmentEmbedding(w.TimeoutCtx, segments)
		w.qdrantLock.Unlock()
		if err != nil {
			err := w.updateRetryCounter(&audioDataElement)
			if err != nil {
				return err
			}

			return err
		}

		slog.Debug("Inserted segment embeddings into Vector-DB for file: " + audioDataElement.AudiofileHash)

		err = w.createNewState(&audioDataElement)
		if err != nil {
			return err
		}
	}

	if audioDataElement.LastSuccessfulStep == globalTypes.StageEmbeddings {
		slog.Debug("Creating summary for file: " + audioDataElement.AudiofileHash)

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
			err := w.updateRetryCounter(&audioDataElement)
			if err != nil {
				return err
			}

			return err
		}

		audioDataElement.AiSummary = summary

		w.dbLock.Lock()
		err = w.db.UpdateAISummary(w.TimeoutCtx, audioDataElement.AudiofileHash, audioDataElement.AiSummary)
		w.dbLock.Unlock()
		if err != nil {
			err := w.updateRetryCounter(&audioDataElement)
			if err != nil {
				return err
			}

			return err
		}

		slog.Debug("Inserted summary into DB for file: " + audioDataElement.AudiofileHash)

		err = w.createNewState(&audioDataElement)
		if err != nil {
			return err
		}
	}

	if audioDataElement.LastSuccessfulStep == globalTypes.StageSummary {
		slog.Debug("Creating keywords for file: " + audioDataElement.AudiofileHash)

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
			err := w.updateRetryCounter(&audioDataElement)
			if err != nil {
				return err
			}

			return err
		}

		r := csv.NewReader(strings.NewReader(keywords))
		record, err := r.Read()
		if err != nil {
			err := w.updateRetryCounter(&audioDataElement)
			if err != nil {
				return err
			}

			return err
		}

		for i := range record {
			record[i] = strings.TrimSpace(record[i])
		}

		audioDataElement.AiKeywords = record

		w.dbLock.Lock()
		err = w.db.UpdateAIKeywords(w.TimeoutCtx, audioDataElement.AudiofileHash, audioDataElement.AiKeywords)
		w.dbLock.Unlock()
		if err != nil {
			err := w.updateRetryCounter(&audioDataElement)
			if err != nil {
				return err
			}

			return err
		}

		slog.Debug("Inserted keywords into DB for file: " + audioDataElement.AudiofileHash)

		err = w.createNewState(&audioDataElement)
		if err != nil {
			return err
		}
	}

	slog.Info("Finished import job for file: " + audioDataElement.AudiofileHash)
	return nil
}

func (w *RoutWorker) createNewState(audioDataElement *globalTypes.AudioDataElement) error {
	audioDataElement.UpdateToNextStage()

	w.dbLock.Lock()
	err := w.db.UpsertBase(w.TimeoutCtx, audioDataElement)
	w.dbLock.Unlock()

	return err
}

func (w *RoutWorker) updateRetryCounter(audioDataElement *globalTypes.AudioDataElement) error {
	audioDataElement.RetryCounter++

	w.dbLock.Lock()
	err := w.db.UpsertBase(w.TimeoutCtx, audioDataElement)
	w.dbLock.Unlock()

	return err
}
