package AudioDataRouter

import (
	"context"
	"encoding/csv"
	"fmt"
	"go_audio_search_api_server/ai"
	"go_audio_search_api_server/globalTypes"
	"go_audio_search_api_server/globalUtils"
	"go_audio_search_api_server/qdrant"
	"go_audio_search_api_server/sqlite"
	"log/slog"
	"strconv"
	"strings"
	"sync"
)

type RoutWorker struct {
	ImportTaskChan chan globalTypes.AudioDataElement
	SearchTaskChan chan globalTypes.SearchRequest
	StopChan       chan bool
	whisper        *ai.WhisperApi
	db             *sqlite.SQLiteStore
	embeddings     *ai.EmbeddingsRequestHandler
	qdrant         *qdrant.Client
	qdrantLock     sync.Mutex
	llmLock        sync.Mutex
	embeddingsLock sync.Mutex
	whisperLock    sync.Mutex
	dbLock         sync.Mutex
}

func NewRoutWorker(databasePath string, workerAmount uint8) *RoutWorker {
	db, err := sqlite.Open(databasePath)
	if err != nil {
		slog.Error(err.Error())
	}

	client, err := qdrant.NewClient("AudioSegments")
	if err != nil {
		slog.Error("Failed to connect to Qdrant: " + err.Error())
		panic("Failed to connect to Qdrant")
	}

	worker := RoutWorker{
		ImportTaskChan: make(chan globalTypes.AudioDataElement),
		StopChan:       make(chan bool),
		whisper:        ai.NewWhisperApi(45.0),
		db:             db,
		embeddings:     ai.NewEmbeddingsRequestHandler(),
		qdrant:         client,
	}

	for i := uint8(0); i < workerAmount; i++ {
		go worker.ProcessChanInputs()
	}

	return &worker
}

func (w *RoutWorker) ProcessChanInputs() {
	for {
		select {
		case inputElement := <-w.ImportTaskChan:
			if inputElement.RetryCounter > 10 {
				slog.Error("Audio file element has been retried more than 10 times, skipping: " + inputElement.AudiofileHash)
				continue
			}

			if !inputElement.FileSavedOnDisk {
				slog.Info("Creating new audio file on disk")

				err := saveAudiofileElementToDisk(inputElement, w.ImportTaskChan)
				if err != nil {
					slog.Error(err.Error())
					inputElement.RetryCounter++
					w.ImportTaskChan <- inputElement
					continue
				}

				slog.Info("Created audio file with hash: " + inputElement.AudiofileHash)

			} else if !inputElement.InitInsertedInDB {
				slog.Info("Inserting new audio file into DB: " + inputElement.AudiofileHash)
				w.dbLock.Lock()

				ctx := context.Background()
				err := w.db.UpsertBase(ctx, &inputElement)

				if err != nil {
					slog.Error("Error inserting audio file into DB: " + err.Error())
					inputElement.RetryCounter++
					w.ImportTaskChan <- inputElement
					continue
				}

				w.dbLock.Unlock()

				slog.Info("Inserted audio file into DB: " + inputElement.AudiofileHash)

				inputElement.InitInsertedInDB = true

				w.ImportTaskChan <- inputElement

			} else if !inputElement.FullTranscriptInDB {
				slog.Info("Creating Transcript for file: " + inputElement.AudiofileHash)

				w.whisperLock.Lock()

				ctx := context.Background()
				result, err := w.whisper.Transcribe(ctx, inputElement.DownloadPath)
				if err != nil {
					slog.Error("Error transcribing audio file: " + err.Error())
					inputElement.RetryCounter++
					w.ImportTaskChan <- inputElement
					continue
				}

				w.whisperLock.Unlock()

				slog.Info(fmt.Sprintf("Transcript for file is done (Transcript Len: %d): %s", len(result.Transcript), inputElement.AudiofileHash))

				inputElement.TranscriptFull = result.Transcript

				slog.Info(fmt.Sprintf("Creating Segments for file: %s", inputElement.AudiofileHash))

				for idx, segment := range result.Segments {
					hashInput := fmt.Sprintf("%s-%f-%f-%s", inputElement.AudiofileHash, segment.Start, segment.End, segment.Transcript)
					builtSegment := globalTypes.SegmentElement{
						AudiofileHash: inputElement.AudiofileHash,
						StartInSec:    segment.Start,
						EndInSec:      segment.End,
						Transcript:    segment.Transcript,
						SegmentHash:   globalUtils.StringSha256Hex(hashInput),
					}

					slog.Debug(fmt.Sprintf("Created Segment %d/%d (Len in sec: %.2f): %s", idx, len(result.Segments), segment.End-segment.Start, inputElement.AudiofileHash))

					inputElement.SegmentElements = append(inputElement.SegmentElements, builtSegment)
				}

				w.dbLock.Lock()

				ctx = context.Background()

				err = w.db.UpdateTranscriptFull(ctx, inputElement.AudiofileHash, inputElement.TranscriptFull)
				if err != nil {
					slog.Error("Error updating full transcript in DB: " + err.Error())
					inputElement.RetryCounter++
					w.ImportTaskChan <- inputElement
					continue
				}

				slog.Info("Inserted transcript into into DB for file: " + inputElement.AudiofileHash)

				err = w.db.InsertSegmentsUpsert(ctx, inputElement.AudiofileHash, inputElement.SegmentElements)
				if err != nil {
					slog.Error("Error inserting audio segments in DB: " + err.Error())
					inputElement.RetryCounter++
					w.ImportTaskChan <- inputElement
					continue
				}

				w.dbLock.Unlock()

				slog.Info("Inserted segments into into DB for file: " + inputElement.AudiofileHash)

				inputElement.FullTranscriptInDB = true
				inputElement.AllSegmentsInDB = true

				w.ImportTaskChan <- inputElement

			} else if !inputElement.SegmentsEmbeddingCreated {
				slog.Info("Creating Embeddings for Segments for file: " + inputElement.AudiofileHash)

				w.embeddingsLock.Lock()

				segments := make([]globalTypes.SegmentElement, len(inputElement.SegmentElements))

				for _, segment := range inputElement.SegmentElements {
					embedding, err := w.embeddings.CreateEmbedding(segment.Transcript)
					if err != nil {
						slog.Error("Error creating embeddings for file: " + inputElement.AudiofileHash + err.Error())
						inputElement.RetryCounter++
						w.ImportTaskChan <- inputElement
						continue
					}
					segment.TranscriptEmbedding = embedding
					segments = append(segments, segment)
				}

				w.embeddingsLock.Unlock()

				slog.Info(strconv.FormatInt(int64(len(segments)), 10) + " embeddings Segments created for file: " + inputElement.AudiofileHash)

				w.dbLock.Lock()

				ctx := context.Background()
				err := w.qdrant.UpsertSegmentEmbedding(ctx, segments)
				if err != nil {
					slog.Error("Error inserting audio segments in Vector-DB: " + err.Error())
					inputElement.RetryCounter++
					w.ImportTaskChan <- inputElement
					continue
				}

				w.dbLock.Unlock()

				slog.Info("Inserted segment embeddings into into DB for file: " + inputElement.AudiofileHash)

				inputElement.SegmentsEmbeddingCreated = true

				w.ImportTaskChan <- inputElement

			} else if !inputElement.AISummaryInDB {
				w.llmLock.Lock()

				_, SummarySysPrompt := ai.GetSysPrompts(inputElement.AudiofileHash)

				summary, err := ai.OllamaRequest(
					globalUtils.LoadEnv("LLM_MODEL"),
					globalUtils.LoadEnv("OLLAMA_API_URL")+"/api/chat",
					SummarySysPrompt,
					inputElement.TranscriptFull,
				)

				w.llmLock.Unlock()

				if err != nil {
					slog.Error("Error creating summary for file: " + inputElement.AudiofileHash + ", " + err.Error())
					inputElement.RetryCounter++
					w.ImportTaskChan <- inputElement
					continue
				}

				inputElement.AiSummary = summary

				w.dbLock.Lock()

				ctx := context.Background()

				err = w.db.UpdateAISummary(ctx, inputElement.AudiofileHash, inputElement.AiSummary)
				if err != nil {
					slog.Error("Error updating summary in DB: " + err.Error())
					inputElement.RetryCounter++
					w.ImportTaskChan <- inputElement
					continue
				}

				w.dbLock.Unlock()

				inputElement.AISummaryInDB = true

				w.ImportTaskChan <- inputElement

			} else if !inputElement.AIKeywordsInDB {
				w.llmLock.Lock()

				keywordSysPrompt, _ := ai.GetSysPrompts(inputElement.AudiofileHash)

				keywords, err := ai.OllamaRequest(
					globalUtils.LoadEnv("LLM_MODEL"),
					globalUtils.LoadEnv("OLLAMA_API_URL")+"/api/chat",
					keywordSysPrompt,
					inputElement.TranscriptFull,
				)

				w.llmLock.Unlock()

				if err != nil {
					slog.Error("Error creating keywords for file: " + inputElement.AudiofileHash + ", " + err.Error())
					inputElement.RetryCounter++
					w.ImportTaskChan <- inputElement
					continue
				}

				r := csv.NewReader(strings.NewReader(keywords))
				record, err := r.Read()
				if err != nil {
					slog.Error("AI returned keywords in unexpected format for file: " + inputElement.AudiofileHash + ", " + err.Error())
					inputElement.RetryCounter++
					w.ImportTaskChan <- inputElement
					continue
				}

				for i := range record {
					record[i] = strings.TrimSpace(record[i])
				}

				inputElement.AiKeywords = record

				w.dbLock.Lock()

				ctx := context.Background()

				err = w.db.UpdateAIKeywords(ctx, inputElement.AudiofileHash, inputElement.AiKeywords)
				if err != nil {
					slog.Error("Error updating Keywords in DB: " + err.Error())
					inputElement.RetryCounter++
					w.ImportTaskChan <- inputElement
					continue
				}

				w.dbLock.Unlock()

				inputElement.AIKeywordsInDB = true
				inputElement.FullyCompleted = true

				w.ImportTaskChan <- inputElement

			} else if inputElement.FullyCompleted {
				slog.Info("Processing fully completed audio file: " + inputElement.AudiofileHash)
			} else {
				slog.Warn("Received audio file element with unknown state: " + inputElement.AudiofileHash)
			}
		case inputElement := <-w.SearchTaskChan:

			//TODO: Implement the search logic
			// Build Response
			var response = globalTypes.SearchResponse{}

			// Find Fts5 Candidates
			w.dbLock.Lock()

			ctx := context.Background()

			candidates, err := w.db.FTS5Candidates(
				ctx,
				inputElement.Fts5Query,
				10,
				inputElement.Category,
				inputElement.StartTimePeriodIso,
				inputElement.EndTimePeriodIso,
			)

			if err != nil {
				response.Err = "Error finding FTS5 candidates for query \"" + inputElement.Fts5Query + "\": " + err.Error()
				response.Ok = false

				slog.Error(response.Err)
				inputElement.BackendResponseChan <- response

				continue
			}

			if len(candidates) == 0 {
				slog.Warn("No FTS5 candidates found for query: " + inputElement.Fts5Query)

			}

			w.dbLock.Unlock()

			// Extract All Segment IDs
			segmentIds := make([]string, len(candidates))
			for idx, candidate := range candidates {
				segmentIds[idx] = candidate.SegmentHash
			}

			// Create Embedding of segmantic search
			w.embeddingsLock.Lock()

			embedding, err := w.embeddings.CreateEmbedding(inputElement.SemanticSearchQuery)
			if err != nil {
				response.Err = "Error creating embedding for semantic search for query \"" + inputElement.SemanticSearchQuery + "\": " + err.Error()
				response.Ok = false

				slog.Error(response.Err)
				inputElement.BackendResponseChan <- response

				continue
			}

			w.embeddingsLock.Unlock()

			// Do qdrant requests get Top K
			w.qdrantLock.Lock()

			segments, err := w.qdrant.RerankCandidatesByHashes(
				ctx,
				embedding,
				segmentIds,
				inputElement.MaxSegmentReturn,
			)

			if err != nil {
				response.Err = "Error while searching qdrant candidates for query: " + inputElement.SemanticSearchQuery + err.Error()
				response.Ok = false

				slog.Error(response.Err)
				inputElement.BackendResponseChan <- response

				continue
			}

			if len(segments) == 0 {
				response.Err = "No candidates found in qdrant for query: " + inputElement.SemanticSearchQuery
				response.Ok = false

				slog.Error(response.Err)
				inputElement.BackendResponseChan <- response

				continue
			}

			w.qdrantLock.Unlock()

			// Extract Audio File Hashes from Top K segments
			audioFileHashes := make(map[string]bool)
			for _, segment := range segments {
				audioFileHashes[segment.AudiofileHash] = true
			}

			// Load Full Audio File data
			w.dbLock.Lock()

			var relatedAudioElements []globalTypes.SearchAudioData
			for audioFileHash := range audioFileHashes {
				audioData, err := w.db.GetSearchAudioDataByHash(ctx, audioFileHash)
				if err != nil {
					slog.Error("Error loading audio file data for hash: " + audioFileHash + ", error: " + err.Error())
					relatedAudioElements = append(relatedAudioElements, globalTypes.SearchAudioData{
						AudiofileHash:  audioFileHash,
						Error:          "Error loading audio file data for this audio hash",
						Title:          "",
						RecordingDate:  "",
						DurationInSec:  0,
						TranscriptFull: "",
						UserSummary:    "",
						AiKeywords:     nil,
						AiSummary:      "",
					})
					continue
				}

				relatedAudioElements = append(relatedAudioElements, *audioData)
			}

			// Load Top K segment
			var fullSegmentElements []globalTypes.SearchSegmentData
			for _, segment := range segments {
				fullSegmentData, err := w.db.GetSegmentByHash(ctx, segment.SegmentHash)
				if err != nil {
					fullSegmentElements = append(fullSegmentElements, globalTypes.SearchSegmentData{
						SegmentHash:   segment.SegmentHash,
						AudiofileHash: segment.AudiofileHash,
						Error:         "Error loading full segment data for segment hash: " + segment.SegmentHash + ", error: " + err.Error(),
						BM25:          segment.BM25,
						QueryScore:    segment.QueryScore,
						Transcript:    "",
						StartInSec:    0,
						EndInSec:      0,
					})
					slog.Error(fullSegmentData.Error)
					continue
				}

				if fullSegmentData != nil {
					for _, candidate := range candidates {
						if candidate.SegmentHash == segment.SegmentHash {
							fullSegmentData.BM25 = candidate.BM25
							break
						}
					}

					fullSegmentData.QueryScore = segment.QueryScore
					fullSegmentElements = append(fullSegmentElements, *fullSegmentData)
				} else {
					slog.Warn("No full segment data found for segment hash: " + segment.SegmentHash)
					fullSegmentElements = append(fullSegmentElements, globalTypes.SearchSegmentData{
						SegmentHash:   segment.SegmentHash,
						AudiofileHash: segment.AudiofileHash,
						Error:         "No full segment data found for this segment",
						BM25:          segment.BM25,
						QueryScore:    segment.QueryScore,
						Transcript:    "",
						StartInSec:    0,
						EndInSec:      0,
					})
				}

			}

			w.dbLock.Unlock()

			// send response via chan
			response.Ok = true
			response.RelatedAudioData = relatedAudioElements
			response.TopKSegments = fullSegmentElements

			inputElement.BackendResponseChan <- response

			slog.Info("Completed search request for query: " + inputElement.SemanticSearchQuery)

		case <-w.StopChan:
			return
		}
	}
}
