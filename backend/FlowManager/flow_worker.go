package FlowManager

import (
	"context"
	"go_audio_search_api_server/ai"
	"go_audio_search_api_server/globalTypes"
	"go_audio_search_api_server/postgres"
	"go_audio_search_api_server/qdrant"
	"log/slog"
	"sync"
	"time"
)

const opTimeout = 5 * time.Minute

type FlowWorker struct {
	ImportTaskChan chan []*globalTypes.AudioDataElement
	SearchTaskChan chan globalTypes.SearchRequest

	PoolRefillSignal chan struct{}

	AudioDataProcessingJobs chan globalTypes.AudioDataElement

	WorkerWG sync.WaitGroup
	Cancel   context.CancelFunc
	StopCtx  context.Context

	whisper    *ai.WhisperWorker
	db         *postgres.Worker
	embeddings *ai.EmbeddingWorker
	qdrant     *qdrant.Worker
	llm        *ai.LlmWorker
}

func NewWorker(workerAmount uint8) *FlowWorker {
	db, err := postgres.Open()
	if err != nil {
		slog.Error(err.Error())
	}

	client, err := qdrant.New("AudioSegments")
	if err != nil {
		slog.Error("Failed to connect to Qdrant: " + err.Error())
		panic("Failed to connect to Qdrant")
	}

	worker := FlowWorker{
		ImportTaskChan:          make(chan []*globalTypes.AudioDataElement),
		SearchTaskChan:          make(chan globalTypes.SearchRequest),
		PoolRefillSignal:        make(chan struct{}, 1),
		AudioDataProcessingJobs: make(chan globalTypes.AudioDataElement, int(workerAmount)),
		whisper:                 ai.New(45.0),
		db:                      db,
		embeddings:              ai.NewEmbeddingsWorker(),
		qdrant:                  client,
		llm:                     ai.NewLlmWorker(),
	}

	worker.StopCtx, worker.Cancel = context.WithCancel(context.Background())

	go worker.ProcessChanInputs()
	go worker.RunDispatcher()

	for i := uint8(0); i < workerAmount; i++ {
		worker.WorkerWG.Add(1)
		go worker.StartImportAudioDataWorker(int(i))
	}

	slog.Debug("Started Dispatcher worker")

	ctx, cancel := worker.opCtx()
	err = worker.db.ResetProcessingClaims(ctx)
	cancel()

	if err != nil {
		slog.Error("Error resetting processing claims in DB: " + err.Error())
	}

	notify(worker.PoolRefillSignal)

	return &worker
}

func (w *FlowWorker) RunDispatcher() {
	slog.Info("Started Dispatcher FlowWorker")
	defer close(w.AudioDataProcessingJobs)
	for {
		select {
		case <-w.StopCtx.Done():
			return

		case <-w.PoolRefillSignal:
			for {
				if len(w.AudioDataProcessingJobs) >= cap(w.AudioDataProcessingJobs) {
					break
				}

				ctx, cancel := w.opCtx()
				job, err := w.db.ClaimNextAudioForProcessing(ctx)

				cancel()

				if err != nil {
					slog.Error("claim failed: " + err.Error())
					break
				}

				if job == nil {
					break
				}

				select {
				case <-w.StopCtx.Done():
					return
				case w.AudioDataProcessingJobs <- *job:
				}
			}
		}
	}
}

func (w *FlowWorker) ProcessChanInputs() {
	slog.Debug("Started Input ProcessChanInputs FlowWorker")
	for {
		select {
		case <-w.StopCtx.Done():
			w.WorkerWG.Wait()
			slog.Info("Stopped ProcessChanInputs")
			return

		case inputElements, ok := <-w.ImportTaskChan:
			if !ok {
				slog.Info("Stopped ProcessChanInputs - ImportTaskChan closed")
				return
			}

			slog.Debug("Inserting new audio file batch into DB")

			ctx, cancel := w.opCtx()
			err := w.db.UpsertBaseBatch(ctx, inputElements)
			cancel()

			if err != nil {
				slog.Error("Error after Batch inserting audio files into DB: " + err.Error())
			}

			notify(w.PoolRefillSignal)

		case inputElement, ok := <-w.SearchTaskChan:
			if !ok {
				return
			}

			var response = globalTypes.SearchResponse{}

			// FTS5 Kandidaten suchen
			ctx, cancel := w.opCtx()
			candidates, err := w.db.GetPostgresCandidates(
				ctx,
				inputElement.Fts5Query,
				10,
				inputElement.Category,
				inputElement.StartTimePeriodIso,
				inputElement.EndTimePeriodIso,
			)
			cancel()

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

			segmentIds := make([]string, len(candidates))
			for idx, candidate := range candidates {
				segmentIds[idx] = candidate.SegmentHash
			}

			// Embedding erstellen
			embedding, err := w.embeddings.CreateEmbedding(inputElement.SemanticSearchQuery)

			if err != nil {
				response.Err = "Error creating embedding for semantic search for query \"" + inputElement.SemanticSearchQuery + "\": " + err.Error()
				response.Ok = false
				slog.Error(response.Err)
				inputElement.BackendResponseChan <- response
				continue
			}

			// Qdrant Reranking
			ctx, cancel = w.opCtx()
			segments, err := w.qdrant.RerankCandidatesByHashes(
				ctx,
				embedding,
				segmentIds,
				inputElement.MaxSegmentReturn,
			)

			cancel()

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

			// Audio-Dateien laden
			audioFileHashes := make(map[string]bool)
			for _, segment := range segments {
				audioFileHashes[segment.AudiofileHash] = true
			}

			var relatedAudioElements []globalTypes.SearchAudioData
			for audioFileHash := range audioFileHashes {
				ctx, cancel = w.opCtx()
				audioData, err := w.db.GetSearchAudioDataByHash(ctx, audioFileHash)
				cancel()

				if err != nil {
					slog.Error("Error loading audio file data for hash: " + audioFileHash + ", error: " + err.Error())
					relatedAudioElements = append(relatedAudioElements, globalTypes.SearchAudioData{
						AudiofileHash: audioFileHash,
						Error:         "Error loading audio file data for this audio hash",
					})
					continue
				}
				relatedAudioElements = append(relatedAudioElements, *audioData)
			}

			// Top-K Segmente laden
			var fullSegmentElements []globalTypes.SearchSegmentData
			for _, segment := range segments {
				ctx, cancel = w.opCtx()
				fullSegmentData, err := w.db.GetSegmentByHash(ctx, segment.SegmentHash)
				cancel()

				if err != nil {
					slog.Error("Error loading segment: " + segment.SegmentHash + ", error: " + err.Error())
					fullSegmentElements = append(fullSegmentElements, globalTypes.SearchSegmentData{
						SegmentHash:   segment.SegmentHash,
						AudiofileHash: segment.AudiofileHash,
						Error:         "Error loading full segment data: " + err.Error(),
						BM25:          segment.PostgresScore,
						QueryScore:    segment.QueryScore,
					})
					continue
				}

				if fullSegmentData != nil {
					for _, candidate := range candidates {
						if candidate.SegmentHash == segment.SegmentHash {
							fullSegmentData.BM25 = candidate.PostgresScore
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
						BM25:          segment.PostgresScore,
						QueryScore:    segment.QueryScore,
					})
				}
			}

			response.Ok = true
			response.RelatedAudioData = relatedAudioElements
			response.TopKSegments = fullSegmentElements
			inputElement.BackendResponseChan <- response

			slog.Info("Completed search request for query: " + inputElement.SemanticSearchQuery)
		}
	}
}
