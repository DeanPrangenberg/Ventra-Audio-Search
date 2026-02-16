package ai

import (
	"log/slog"
)

type EmbeddingsRequestHandler struct {
	model      string
	requestURL string
}

func NewEmbeddingsRequestHandler() *EmbeddingsRequestHandler {
	slog.Info("Creating new NewEmbeddingsRequestHandler...")
	model := loadEnv("EMBEDDING_MODEL") // z.B. "nomic-embed-text"
	ollama := loadEnv("OLLAMA_API_URL") // z.B. "http://ollama:11434"

	return &EmbeddingsRequestHandler{
		model:      model,
		requestURL: ollama + "/api/embed",
	}
}

func (h *EmbeddingsRequestHandler) CreateEmbedding(text string) ([]float32, error) {
	vec, err := OllamaEmbedRequest(h.model, h.requestURL, text)
	if err != nil {
		return nil, err
	}

	return vec, nil
}
