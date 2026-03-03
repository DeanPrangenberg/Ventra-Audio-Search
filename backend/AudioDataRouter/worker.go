package AudioDataRouter

import (
	"context"
	"go_audio_search_api_server/ai"
	"go_audio_search_api_server/globalTypes"
	"go_audio_search_api_server/qdrant"
	"go_audio_search_api_server/sqlite"
	"log/slog"
	"sync"
	"time"
)

type RoutWorker struct {
	ImportTaskChan chan *[]globalTypes.AudioDataElement
	SearchTaskChan chan globalTypes.SearchRequest

	NewAudioDataSignal      chan struct{}
	AudioDataProcessingJobs chan globalTypes.AudioDataElement

	WorkerWG sync.WaitGroup
	Cancel   context.CancelFunc
	StopCtx  context.Context

	whisper        *ai.WhisperApi
	db             *sqlite.SQLiteStore
	embeddings     *ai.EmbeddingsRequestHandler
	qdrant         *qdrant.Client
	qdrantLock     sync.Mutex
	llmLock        sync.Mutex
	embeddingsLock sync.Mutex
	whisperLock    sync.Mutex
	dbLock         sync.Mutex

	TimeoutCtx    context.Context
	TimeoutCancel context.CancelFunc
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
		ImportTaskChan:          make(chan *[]globalTypes.AudioDataElement),
		SearchTaskChan:          make(chan globalTypes.SearchRequest),
		NewAudioDataSignal:      make(chan struct{}),
		AudioDataProcessingJobs: make(chan globalTypes.AudioDataElement),
		whisper:                 ai.NewWhisperApi(45.0),
		db:                      db,
		embeddings:              ai.NewEmbeddingsRequestHandler(),
		qdrant:                  client,
	}

	worker.StopCtx, worker.Cancel = context.WithCancel(context.Background())
	worker.TimeoutCtx, worker.TimeoutCancel = context.WithTimeout(worker.StopCtx, 5*time.Minute)

	go worker.ProcessChanInputs()
	go worker.RunDispatcher()

	for i := uint8(0); i < workerAmount; i++ {
		worker.WorkerWG.Add(int(i))
		go worker.StartImportAudioDataWorker(int(i))
	}

	return &worker
}

func (w *RoutWorker) RunDispatcher() {
	slog.Debug("Started Dispatcher worker")
	w.dbLock.Lock()
	err := w.db.ResetProcessingClaims(w.TimeoutCtx)
	w.dbLock.Unlock()

	if err != nil {
		slog.Error("Error resetting processing claims in DB: " + err.Error())
	}

	notify(w.NewAudioDataSignal)

	for {
		select {
		case <-w.StopCtx.Done():
			return

		case <-w.NewAudioDataSignal:
			for len(w.AudioDataProcessingJobs) < cap(w.AudioDataProcessingJobs) {

				w.dbLock.Lock()
				job, err := w.db.ClaimNextAudioForProcessing(w.TimeoutCtx)
				w.dbLock.Unlock()

				if err != nil {
					slog.Error("claim failed: " + err.Error())
					continue
				}

				if job == nil {
					continue
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

func (w *RoutWorker) ProcessChanInputs() {
	slog.Debug("Started Input ProcessChanInputs Worker")
	for {
		select {
		case inputElements := <-w.ImportTaskChan:

			for _, element := range *inputElements {
				element.AudiofileHash = element.GetTmpHash()
				element.LastSuccessfulStep = globalTypes.StageReceived

				w.dbLock.Lock()
				slog.Debug("Inserting new audio file into DB: " + element.AudiofileHash)
				//TODO: Add mass import
				err := w.db.UpsertBase(w.TimeoutCtx, &element)
				w.dbLock.Unlock()
				if err != nil {
					slog.Error("Error inserting audio file into DB: " + err.Error())
				}
			}

			notify(w.NewAudioDataSignal)

		case inputElement := <-w.SearchTaskChan:

			// Build Response
			var response = globalTypes.SearchResponse{}

			// Find Fts5 Candidates
			w.dbLock.Lock()

			candidates, err := w.db.FTS5Candidates(
				w.TimeoutCtx,
				inputElement.Fts5Query,
				10,
				inputElement.Category,
				inputElement.StartTimePeriodIso,
				inputElement.EndTimePeriodIso,
			)
			w.dbLock.Unlock()

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

			// Extract All Segment IDs
			segmentIds := make([]string, len(candidates))
			for idx, candidate := range candidates {
				segmentIds[idx] = candidate.SegmentHash
			}

			// Create Embedding of segmantic search
			w.embeddingsLock.Lock()

			embedding, err := w.embeddings.CreateEmbedding(inputElement.SemanticSearchQuery)
			w.embeddingsLock.Unlock()
			if err != nil {
				response.Err = "Error creating embedding for semantic search for query \"" + inputElement.SemanticSearchQuery + "\": " + err.Error()
				response.Ok = false

				slog.Error(response.Err)
				inputElement.BackendResponseChan <- response

				continue
			}

			// Do qdrant requests get Top K
			w.qdrantLock.Lock()

			segments, err := w.qdrant.RerankCandidatesByHashes(
				w.TimeoutCtx,
				embedding,
				segmentIds,
				inputElement.MaxSegmentReturn,
			)
			w.qdrantLock.Unlock()

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

			// Extract Audio File Hashes from Top K segments
			audioFileHashes := make(map[string]bool)
			for _, segment := range segments {
				audioFileHashes[segment.AudiofileHash] = true
			}

			var relatedAudioElements []globalTypes.SearchAudioData
			for audioFileHash := range audioFileHashes {
				w.dbLock.Lock()
				audioData, err := w.db.GetSearchAudioDataByHash(w.TimeoutCtx, audioFileHash)
				w.dbLock.Unlock()
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
				w.dbLock.Lock()
				fullSegmentData, err := w.db.GetSegmentByHash(w.TimeoutCtx, segment.SegmentHash)
				w.dbLock.Unlock()
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

			// send response via chan
			response.Ok = true
			response.RelatedAudioData = relatedAudioElements
			response.TopKSegments = fullSegmentElements

			inputElement.BackendResponseChan <- response

			slog.Info("Completed search request for query: " + inputElement.SemanticSearchQuery)

		case <-w.StopCtx.Done():
			close(w.ImportTaskChan)
			close(w.SearchTaskChan)
			close(w.NewAudioDataSignal)
			close(w.AudioDataProcessingJobs)
			w.TimeoutCancel()
			w.Cancel()
			w.WorkerWG.Wait()
			slog.Info("Stopped importing audio data")
		}
	}
}
