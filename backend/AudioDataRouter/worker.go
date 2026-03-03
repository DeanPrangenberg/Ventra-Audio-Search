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

// FIX: Per-Operation Timeout statt globalem TimeoutCtx
const opTimeout = 30 * time.Second

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
}

// FIX: Helper für per-Operation Context — kein globaler Timeout mehr
func (w *RoutWorker) opCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(w.StopCtx, opTimeout)
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
		ImportTaskChan: make(chan *[]globalTypes.AudioDataElement),
		SearchTaskChan: make(chan globalTypes.SearchRequest),
		// FIX: Buffered mit size 1 — verhindert dropped Signals ohne zu blocken
		NewAudioDataSignal: make(chan struct{}, 1),
		// FIX: Gepuffert mit workerAmount — war unbuffered (cap=0), Loop lief nie
		AudioDataProcessingJobs: make(chan globalTypes.AudioDataElement, int(workerAmount)),
		whisper:                 ai.NewWhisperApi(45.0),
		db:                      db,
		embeddings:              ai.NewEmbeddingsRequestHandler(),
		qdrant:                  client,
	}

	worker.StopCtx, worker.Cancel = context.WithCancel(context.Background())

	go worker.ProcessChanInputs()
	go worker.RunDispatcher()

	for i := uint8(0); i < workerAmount; i++ {
		worker.WorkerWG.Add(1) // FIX: war Add(int(i)) → zählte 0,1,2,3... statt immer 1
		go worker.StartImportAudioDataWorker(int(i))
	}

	slog.Debug("Started Dispatcher worker")

	ctx, cancel := worker.opCtx()
	worker.dbLock.Lock()
	err = worker.db.ResetProcessingClaims(ctx)
	worker.dbLock.Unlock()
	cancel()

	if err != nil {
		slog.Error("Error resetting processing claims in DB: " + err.Error())
	}

	notify(worker.NewAudioDataSignal)

	return &worker
}

func (w *RoutWorker) RunDispatcher() {
	slog.Info("Started Dispatcher Worker")
	// FIX: Sender schließt den Channel — war im Receiver (ProcessChanInputs) → Panic-Risiko
	defer close(w.AudioDataProcessingJobs)
	for {
		select {
		case <-w.StopCtx.Done():
			return

		case <-w.NewAudioDataSignal:
			for {
				if len(w.AudioDataProcessingJobs) >= cap(w.AudioDataProcessingJobs) {
					break
				}

				ctx, cancel := w.opCtx()
				w.dbLock.Lock()
				job, err := w.db.ClaimNextAudioForProcessing(ctx)
				w.dbLock.Unlock()
				cancel()

				if err != nil {
					slog.Error("claim failed: " + err.Error())
					// FIX: break statt continue — verhindert Busy-Loop bei DB-Fehler
					break
				}

				if job == nil {
					// FIX: break statt continue — kein Job verfügbar, nicht weiter loopen
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

func (w *RoutWorker) ProcessChanInputs() {
	slog.Debug("Started Input ProcessChanInputs Worker")
	for {
		select {
		// FIX: Shutdown hier — kein close() mehr auf Channels die wir nicht besitzen
		case <-w.StopCtx.Done():
			w.WorkerWG.Wait()
			slog.Info("Stopped ProcessChanInputs")
			return

		case inputElements, ok := <-w.ImportTaskChan:
			if !ok {
				return
			}

			for _, element := range *inputElements {
				element.AudiofileHash = element.GetTmpHash()
				element.LastSuccessfulStep = globalTypes.StageReceived

				ctx, cancel := w.opCtx()
				w.dbLock.Lock()
				slog.Debug("Inserting new audio file into DB: " + element.AudiofileHash)
				// TODO: Add mass import
				err := w.db.UpsertBase(ctx, &element)
				w.dbLock.Unlock()
				cancel()

				if err != nil {
					slog.Error("Error inserting audio file into DB: " + err.Error())
				}
			}

			notify(w.NewAudioDataSignal)

		case inputElement, ok := <-w.SearchTaskChan:
			if !ok {
				return
			}

			var response = globalTypes.SearchResponse{}

			// FTS5 Kandidaten suchen
			ctx, cancel := w.opCtx()
			w.dbLock.Lock()
			candidates, err := w.db.FTS5Candidates(
				ctx,
				inputElement.Fts5Query,
				10,
				inputElement.Category,
				inputElement.StartTimePeriodIso,
				inputElement.EndTimePeriodIso,
			)
			w.dbLock.Unlock()
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

			// Qdrant Reranking
			ctx, cancel = w.opCtx()
			w.qdrantLock.Lock()
			segments, err := w.qdrant.RerankCandidatesByHashes(
				ctx,
				embedding,
				segmentIds,
				inputElement.MaxSegmentReturn,
			)
			w.qdrantLock.Unlock()
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
				w.dbLock.Lock()
				audioData, err := w.db.GetSearchAudioDataByHash(ctx, audioFileHash)
				w.dbLock.Unlock()
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
				w.dbLock.Lock()
				fullSegmentData, err := w.db.GetSegmentByHash(ctx, segment.SegmentHash)
				w.dbLock.Unlock()
				cancel()

				if err != nil {
					slog.Error("Error loading segment: " + segment.SegmentHash + ", error: " + err.Error())
					fullSegmentElements = append(fullSegmentElements, globalTypes.SearchSegmentData{
						SegmentHash:   segment.SegmentHash,
						AudiofileHash: segment.AudiofileHash,
						Error:         "Error loading full segment data: " + err.Error(),
						BM25:          segment.BM25,
						QueryScore:    segment.QueryScore,
					})
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
