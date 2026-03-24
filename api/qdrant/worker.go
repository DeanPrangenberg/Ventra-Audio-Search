package qdrant

import (
	"context"
	"go_audio_search_api_server/globalUtils"
	"strings"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
)

type Worker struct {
	collectionName string
	client         *qdrant.Client
}

func New(collectionName string) (*Worker, error) {

	host := globalUtils.LoadEnvStr("QDRANT_API_HOST")

	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	host = strings.Split(host, ":")[0]

	client, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: globalUtils.LoadEnvInt("QDRANT_API_PORT_GRPC"),
	})
	if err != nil {
		return nil, err
	}

	err = client.CreateCollection(context.Background(), &qdrant.CreateCollection{
		CollectionName: collectionName,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     globalUtils.LoadEnvUInt64("EMBEDDING_MODEL_DIM"),
			Distance: qdrant.Distance_Cosine,
		}),
	})

	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return nil, err
	}

	return &Worker{
		collectionName: collectionName,
		client:         client,
	}, nil
}

func segmentHashToPointID(segmentHash string) *qdrant.PointId {
	segmentHash = strings.TrimSpace(segmentHash)

	id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(segmentHash)) // stabil
	return qdrant.NewIDUUID(id.String())
}
