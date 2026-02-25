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

func (c *Client) RerankCandidatesByHashes(
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

	resp, err := c.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: c.collectionName,
		Query:          qdrant.NewQuery(queryVec...),
		Limit:          &n,
		Filter:         filter,
		WithPayload:    qdrant.NewWithPayloadInclude("SegmentHash"),
	})

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
