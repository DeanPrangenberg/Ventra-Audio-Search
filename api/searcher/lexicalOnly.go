package searcher

import (
	"fmt"
	"go_audio_search_api_server/globalTypes"
	"log/slog"
)

func (w *Worker) lexicalSearch(searchQuery globalTypes.SearchRequest) *globalTypes.SearchResponse {
	slog.Debug(fmt.Sprintf("Starting lexical only search for TsQuery=%s", searchQuery.TsQuery))
	var response = globalTypes.SearchResponse{}

	// Find Ts Candidates
	ctx, cancel := w.opCtx()
	candidates, err := w.postgres.GetPostgresCandidates(
		ctx,
		searchQuery.TsQuery,
		int(searchQuery.MaxSegmentReturn),
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

	// Load data for Top K
	var fullSegmentElements []globalTypes.SearchSegmentData
	for _, segment := range candidates {
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

	slog.Info(fmt.Sprintf("Completed lexical only search for TsQuery=%s", searchQuery.TsQuery))

	return &response
}
