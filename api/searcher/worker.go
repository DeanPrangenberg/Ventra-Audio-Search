package searcher

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

type Worker struct {
	workerWG *sync.WaitGroup
	stopCtx  context.Context
	qdrant   *qdrant.Worker
	postgres *postgres.Worker
	embedder *ai.EmbeddingWorker
}

func NewWorker(ctx context.Context, wg *sync.WaitGroup, qdrant *qdrant.Worker, postgres *postgres.Worker, embedder *ai.EmbeddingWorker) *Worker {

	worker := Worker{
		postgres: postgres,
		qdrant:   qdrant,
		workerWG: wg,
		stopCtx:  ctx,
		embedder: embedder,
	}

	return &worker
}

func (w *Worker) Search(searchQuery globalTypes.SearchRequest) *globalTypes.SearchResponse {
	if searchQuery.TsQuery != "" && searchQuery.SemanticSearchQuery != "" {
		return w.normalSearch(searchQuery)
	}

	if searchQuery.TsQuery != "" && searchQuery.SemanticSearchQuery == "" {
		return w.lexicalSearch(searchQuery)
	}

	if searchQuery.SemanticSearchQuery != "" && searchQuery.TsQuery == "" {
		return w.semanticSearch(searchQuery)
	}

	slog.Error("Received invalid")
	return &globalTypes.SearchResponse{
		Err: "Internal server error: Received invalid query, both TsQuery and SemanticSearchQuery are empty",
		Ok:  false,
	}
}
