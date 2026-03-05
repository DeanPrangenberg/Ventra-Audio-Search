package importer

import (
	"context"
	"go_audio_search_api_server/ai"
	"go_audio_search_api_server/globalTypes"
	"go_audio_search_api_server/postgres"
	"go_audio_search_api_server/qdrant"
	"log/slog"
	"sync"
)

type Worker struct {
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

func NewWorker(qdrant *qdrant.Worker, postgres *postgres.Worker, workerAmount uint8) *Worker {

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

func (w *FlowWorker) StartImportJobDispatcher(idx int) {
	slog.Debug("import Stage Dispatcher started", "worker", idx)

	for {
		select {
		case <-w.StopCtx.Done():
			slog.Debug("import worker stopped", "worker", idx, "reason", "stop_ctx_done")
			return

		case audioDataElement := <-w.AudioDataProcessingJobs:

		}
	}
}
