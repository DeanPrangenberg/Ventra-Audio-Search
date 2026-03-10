package importer

import (
	"context"
	"go_audio_search_api_server/ai"
	"go_audio_search_api_server/globalTypes"
	"go_audio_search_api_server/globalUtils"
	"go_audio_search_api_server/postgres"
	"go_audio_search_api_server/qdrant"
	"log/slog"
	"sync"
	"time"
)

const opTimeout = 5 * time.Minute
const maxRetryCount = 10

type Worker struct {
	PoolRefillSignal *globalUtils.NoneStackingEvent

	persistFileBuffer      chan *globalTypes.AudioDataElement
	transcriptAudioBuffer  chan *globalTypes.AudioDataElement
	createEmbeddingsBuffer chan *globalTypes.AudioDataElement
	genAiDataBuffer        chan *globalTypes.AudioDataElement

	WorkerWG *sync.WaitGroup
	StopCtx  context.Context

	whisper    *ai.WhisperWorker
	postgres   *postgres.Worker
	embeddings *ai.EmbeddingWorker
	qdrant     *qdrant.Worker
	llm        *ai.LlmWorker
}

func NewWorker(ctx context.Context, wg *sync.WaitGroup, qdrant *qdrant.Worker, postgres *postgres.Worker, embedder *ai.EmbeddingWorker, poolRefillSignal *globalUtils.NoneStackingEvent) *Worker {

	whisperReplicas := globalUtils.LoadEnvInt("WHISPER_REPLICAS")

	worker := Worker{
		PoolRefillSignal:       poolRefillSignal,
		StopCtx:                ctx,
		persistFileBuffer:      make(chan *globalTypes.AudioDataElement, 20),
		transcriptAudioBuffer:  make(chan *globalTypes.AudioDataElement, whisperReplicas*2),
		createEmbeddingsBuffer: make(chan *globalTypes.AudioDataElement, 4),
		genAiDataBuffer:        make(chan *globalTypes.AudioDataElement, 4),
		whisper:                ai.New(45.0),
		postgres:               postgres,
		embeddings:             embedder,
		qdrant:                 qdrant,
		llm:                    ai.NewLlmWorker(),
		WorkerWG:               wg,
	}

	ctx, cancel := worker.opCtx()
	err := worker.postgres.ResetProcessingClaims(ctx)
	cancel()

	if err != nil {
		slog.Error("Error resetting processing claims in DB: " + err.Error())
	}

	worker.startPersistFilePool(10)
	worker.startTranscriptAudioPool(uint(2 * whisperReplicas))
	worker.startCreateEmbeddingsPool(2)
	worker.startGenerateAiDataPool(2)

	go worker.startImportJobDispatcher()

	worker.PoolRefillSignal.Trigger()

	return &worker
}

func (w *Worker) startImportJobDispatcher() {
	slog.Debug("Started Importer Job Dispatcher")

	for {
		select {
		case <-w.StopCtx.Done():
			slog.Debug("Importer Job Dispatcher stopped", "reason", "stop_ctx_done")
			return

		case <-w.PoolRefillSignal.Reader():
			slog.Debug("Refilling importer job buffers")

			w.refillBuffer(w.persistFileBuffer, globalTypes.StageQueued)
			w.refillBuffer(w.transcriptAudioBuffer, globalTypes.StageFilePersisted)
			w.refillBuffer(w.createEmbeddingsBuffer, globalTypes.StageTranscribed)

			if globalUtils.LoadEnvStr("DEACTIVATE_LLM") != "true" {
				w.refillBuffer(w.genAiDataBuffer, globalTypes.StageEmbedded)
			}
		}
	}
}

func (w *Worker) refillBuffer(
	buffer chan *globalTypes.AudioDataElement,
	stage globalTypes.ProcessingStage,
) {
	space := cap(buffer) - len(buffer)
	if space <= 0 {
		return
	}

	ctx, cancel := w.opCtx()
	audioDataElements, err := w.postgres.ClaimNextAudioForProcessing(ctx, stage, uint64(space))
	cancel()

	if err != nil {
		slog.Error("claim failed", "stage", stage, "err", err)
		return
	}

	for _, audioDataElement := range audioDataElements {
		buffer <- audioDataElement
	}
}
