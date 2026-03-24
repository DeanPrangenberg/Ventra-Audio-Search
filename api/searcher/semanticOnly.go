package searcher

import (
	"fmt"
	"go_audio_search_api_server/globalTypes"
	"log/slog"
)

func (w *Worker) semanticSearch(searchQuery globalTypes.SearchRequest) *globalTypes.SearchResponse {
	slog.Debug(fmt.Sprintf("Starting semantic search for SemanticQuery=%s", searchQuery.SemanticSearchQuery))
	var response = globalTypes.SearchResponse{}

	// Creating Query Embedding
	embedding, err := w.embedder.CreateEmbedding(searchQuery.SemanticSearchQuery)

	if err != nil {
		response.Err = "Error creating embedding for semantic search for query \"" + searchQuery.SemanticSearchQuery + "\": " + err.Error()
		response.Ok = false
		slog.Error(response.Err)
		return &response
	}

	// Qdrant Reranking
	ctx, cancel := w.opCtx()
	segments, err := w.qdrant.QueryCandidates(
		ctx,
		embedding,
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

	// Load data for Top K
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

		fullSegmentElements = append(fullSegmentElements, *fullSegmentData)
	}

	// Load connected audio data
	var audioFileHashes []string
	for _, segment := range fullSegmentElements {
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
				Error:         "Error loading audio file data for this audio hash: " + err.Error(),
			})
			continue
		}
		relatedAudioElements = append(relatedAudioElements, *audioData)
	}

	response.Ok = true
	response.RelatedAudioData = relatedAudioElements
	response.TopKSegments = fullSegmentElements

	slog.Info(fmt.Sprintf("Completed semantic search for SemanticQuery=%s", searchQuery.SemanticSearchQuery))

	return &response
}
