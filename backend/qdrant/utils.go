package qdrant

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
)

func loadVectorDim() uint64 {
	v, ok := os.LookupEnv("EMBEDDING_MODEL_DIM")
	if !ok || v == "" {
		panic(fmt.Sprintf("missing env var: %s", "EMBEDDING_MODEL_DIM"))
	}

	u, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		panic(fmt.Sprintf("invalid env var: %s, must be a positive integer like \"768\"", "EMBEDDING_MODEL_DIM"))
	}

	return u
}

func loadEnvInt(key string) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		panic(fmt.Sprintf("missing env var: %s", "EMBEDDING_MODEL_DIM"))
	}

	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		panic(fmt.Sprintf("invalid env var: %s, must be a positive integer like \"768\"", "EMBEDDING_MODEL_DIM"))
	}

	return int(i)
}

func loadEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		panic(fmt.Sprintf("missing env var: %s", key))
	}
	return v
}

func segmentHashToPointID(segmentHash string) *qdrant.PointId {
	segmentHash = strings.TrimSpace(segmentHash)

	// 1) Falls segmentHash bereits UUID ist -> direkt nehmen
	if _, err := uuid.Parse(segmentHash); err == nil {
		return qdrant.NewIDUUID(segmentHash)
	}

	// 2) Sonst deterministische UUID v5 aus segmentHash
	id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(segmentHash)) // stabil
	return qdrant.NewIDUUID(id.String())
}
