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
	slog.Debug("Started Input Search Worker")
	var response = globalTypes.SearchResponse{}

	// FTS5 Kandidaten suchen
	ctx, cancel := w.opCtx()
	candidates, err := w.postgres.GetPostgresCandidates(
		ctx,
		searchQuery.TsQuery,
		100,
		searchQuery.Category,
		searchQuery.StartTimePeriodIso,
		searchQuery.EndTimePeriodIso,
	)
	cancel()

	if err != nil {
		response.Err = "Error finding FTS5 candidates for query \"" + searchQuery.TsQuery + "\": " + err.Error()
		response.Ok = false
		slog.Error(response.Err)
		return &response
	}

	if len(candidates) == 0 {
		slog.Warn("No FTS5 candidates found for query: " + searchQuery.TsQuery)
	}

	segmentIds := make([]string, len(candidates))
	for idx, candidate := range candidates {
		segmentIds[idx] = candidate.SegmentHash
	}

	// Embedding erstellen
	embedding, err := w.embedder.CreateEmbedding(searchQuery.SemanticSearchQuery)

	if err != nil {
		response.Err = "Error creating embedding for semantic search for query \"" + searchQuery.SemanticSearchQuery + "\": " + err.Error()
		response.Ok = false
		slog.Error(response.Err)
		return &response
	}

	// Qdrant Reranking
	ctx, cancel = w.opCtx()
	segments, err := w.qdrant.RerankCandidatesByHashes(
		ctx,
		embedding,
		segmentIds,
		searchQuery.MaxSegmentReturn,
	)

	cancel()

	if err != nil {
		response.Err = "Error while searching qdrant candidates for query: " + searchQuery.SemanticSearchQuery + err.Error()
		response.Ok = false
		slog.Error(response.Err)
		return &response
	}

	if len(segments) == 0 {
		response.Err = "No candidates found in qdrant for query: " + searchQuery.SemanticSearchQuery
		response.Ok = false
		slog.Error(response.Err)
		return &response
	}

	// Audio-Dateien laden
	var audioFileHashes []string
	for _, segment := range segments {
		contains := false
		for _, savedHash := range audioFileHashes {
			if savedHash == segment.AudiofileHash {
				contains = true
				break
			}
		}
		if !contains {
			audioFileHashes = append(audioFileHashes, segment.AudiofileHash)
		}
	}

	var relatedAudioElements []globalTypes.SearchAudioData
	for _, audioFileHash := range audioFileHashes {
		ctx, cancel = w.opCtx()
		audioData, err := w.postgres.GetSearchAudioDataByHash(ctx, audioFileHash)
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
		fullSegmentData, err := w.postgres.GetSegmentByHash(ctx, segment.SegmentHash)
		cancel()

		if err != nil {
			slog.Error("Error loading segment: " + segment.SegmentHash + ", error: " + err.Error())
			fullSegmentElements = append(fullSegmentElements, globalTypes.SearchSegmentData{
				SegmentHash:   segment.SegmentHash,
				AudiofileHash: segment.AudiofileHash,
				Error:         "Error loading full segment data: " + err.Error(),
				TsScore:       segment.TsScore,
				QueryScore:    segment.QueryScore,
			})
			continue
		}

		if fullSegmentData != nil {
			for _, candidate := range candidates {
				if candidate.SegmentHash == segment.SegmentHash {
					fullSegmentData.TsScore = candidate.TsScore
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
				TsScore:       segment.TsScore,
				QueryScore:    segment.QueryScore,
			})
		}
	}

	response.Ok = true
	response.RelatedAudioData = relatedAudioElements
	response.TopKSegments = fullSegmentElements

	slog.Info("Completed search request for query: " + searchQuery.SemanticSearchQuery)

	return &response
}
