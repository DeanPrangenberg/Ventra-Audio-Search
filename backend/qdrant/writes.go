package qdrant

import (
	"context"
	"go_audio_search_api_server/globalTypes"
	"log/slog"

	"github.com/qdrant/go-client/qdrant"
)

func (w *Worker) UpsertSegmentEmbeddings(ctx context.Context, elements *[]globalTypes.SegmentElement) error {
	var points []*qdrant.PointStruct

	for _, element := range *elements {
		point := &qdrant.PointStruct{
			Id:      segmentHashToPointID(element.SegmentHash),
			Vectors: qdrant.NewVectors(element.TranscriptEmbedding...),
			Payload: qdrant.NewValueMap(map[string]any{"SegmentHash": element.SegmentHash}),
		}

		points = append(points, point)
	}

	operationInfo, err := w.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: w.collectionName,
		Points:         points,
	})

	if err != nil {
		return err
	}

	slog.Info("Upserted points to Qdrant", "operationInfo", operationInfo)

	return nil
}
