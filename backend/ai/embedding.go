package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go_audio_search_api_server/globalUtils"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type EmbeddingWorker struct {
	model      string
	requestURL string
	lock       sync.Mutex
}

func NewEmbeddingsWorker() *EmbeddingWorker {
	slog.Info("Creating new NewEmbeddingsWorker...")
	model := globalUtils.LoadEnvStr("EMBEDDING_MODEL")
	ollama := globalUtils.LoadEnvStr("OLLAMA_API_URL")

	return &EmbeddingWorker{
		model:      model,
		requestURL: ollama + "/api/embed",
	}
}

func (h *EmbeddingWorker) CreateEmbedding(text string) ([]float32, error) {
	slog.Info("Requesting Ollama embed", "model", h.model)
	client := &http.Client{Timeout: 1200 * time.Second}

	reqBody := ollamaEmbedReq{
		Model: h.model,
		Input: text,
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	h.lock.Lock()
	resp, err := client.Post(h.requestURL, "application/json", bytes.NewReader(b))
	h.lock.Unlock()

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ollama embed failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var out ollamaEmbedResp
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if out.Error != "" {
		return nil, errors.New(out.Error)
	}
	if len(out.Embeddings) == 0 || len(out.Embeddings[0]) == 0 {
		return nil, fmt.Errorf("ollama embed returned empty embeddings: body=%s", string(body))
	}

	vec64 := out.Embeddings[0]
	vec32 := make([]float32, len(vec64))
	for i, v := range vec64 {
		vec32[i] = float32(v)
	}

	return vec32, nil
}
