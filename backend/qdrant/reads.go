package qdrant

import (
	"context"
	"errors"
	"go_audio_search_api_server/globalTypes"

	"github.com/qdrant/go-client/qdrant"
)

func (w *Worker) RerankCandidatesByHashes(
	ctx context.Context,
	queryVec []float32,
	candidateSegmentHashes []string,
	n uint64,
) ([]globalTypes.SegmentElement, error) {
	if len(queryVec) == 0 {
		return nil, errors.New("queryVec empty")
	}

	if len(candidateSegmentHashes) == 0 {
		return []globalTypes.SegmentElement{}, nil
	}

	if n == 0 {
		n = 10
	}

	// Candidate IDs
	ids := make([]*qdrant.PointId, 0, len(candidateSegmentHashes))
	for _, h := range candidateSegmentHashes {
		ids = append(ids, segmentHashToPointID(h))
	}

	filter := &qdrant.Filter{
		Must: []*qdrant.Condition{
			{
				ConditionOneOf: &qdrant.Condition_HasId{
					HasId: &qdrant.HasIdCondition{
						HasId: ids,
					},
				},
			},
		},
	}

	w.lock.Lock()
	resp, err := w.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: w.collectionName,
		Query:          qdrant.NewQuery(queryVec...),
		Limit:          &n,
		Filter:         filter,
		WithPayload:    qdrant.NewWithPayloadInclude("SegmentHash"),
	})
	w.lock.Unlock()

	if err != nil {
		return nil, err
	}

	out := make([]globalTypes.SegmentElement, 0, len(resp))
	for _, p := range resp {
		segment := globalTypes.SegmentElement{
			SegmentHash: p.Payload["SegmentHash"].GetStringValue(),
			QueryScore:  p.Score,
		}
		out = append(out, segment)
	}

	return out, nil
}
