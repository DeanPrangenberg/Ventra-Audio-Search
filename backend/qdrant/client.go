package qdrant

import (
	"context"
	"errors"
	"go_audio_search_api_server/globalTypes"
	"log/slog"
	"strings"

	"github.com/qdrant/go-client/qdrant"
)

type Client struct {
	collectionName string
	client         *qdrant.Client
}

func NewClient(collectionName string) (*Client, error) {

	host := loadEnv("QDRANT_API_HOST")

	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	host = strings.Split(host, ":")[0] // Port wegwerfen, weil Port separat kommt

	client, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: loadEnvInt("QDRANT_API_PORT_GRPC"),
	})
	if err != nil {
		return nil, err
	}

	err = client.CreateCollection(context.Background(), &qdrant.CreateCollection{
		CollectionName: collectionName,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     loadVectorDim(),
			Distance: qdrant.Distance_Cosine,
		}),
	})

	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return nil, err
	}

	return &Client{
		collectionName: collectionName,
		client:         client,
	}, nil
}

func (c *Client) UpsertSegmentEmbedding(ctx context.Context, elements []globalTypes.SegmentElement) error {
	var points []*qdrant.PointStruct

	for _, element := range elements {
		point := &qdrant.PointStruct{
			Id:      segmentHashToPointID(element.SegmentHash),
			Vectors: qdrant.NewVectors(element.TranscriptEmbedding...),
			Payload: qdrant.NewValueMap(map[string]any{"SegmentHash": element.SegmentHash}),
		}

		points = append(points, point)
	}

	operationInfo, err := c.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: c.collectionName,
		Points:         points,
	})

	if err != nil {
		return err
	}

	slog.Info("Upserted points to Qdrant", "operationInfo", operationInfo)

	return nil
}

func (c *Client) FindTopNSegment(ctx context.Context, queryVec []float32, n uint64) ([]map[string]any, error) {
	if len(queryVec) == 0 {
		return nil, errors.New("queryVec empty")
	}
	if n == 0 {
		n = 10
	}

	var resp []*qdrant.ScoredPoint
	var err error

	resp, err = c.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: c.collectionName,
		Query:          qdrant.NewQuery(queryVec...),
		Limit:          &n,
		WithPayload:    qdrant.NewWithPayloadInclude("SegmentHash"),
	})

	if err != nil {
		return nil, err
	}

	out := make([]map[string]any, 0, len(resp))
	for _, element := range resp {
		tmp := make(map[string]any)
		if element.Payload != nil {
			for key, value := range element.Payload {
				tmp[key] = value
			}
		}

		tmp["score"] = element.Score

		out = append(out, tmp)
	}

	if len(out) == 0 {
		return out, nil
	}

	return out, nil
}
