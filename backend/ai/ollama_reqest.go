package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

func OllamaRequest(model string, requestUrl string, sysPrompt string, userPrompt string) (string, error) {
	slog.Info("Requesting Ollama model", "model", model)

	client := &http.Client{Timeout: 1200 * time.Second}

	reqBody := ChatReq{
		Model: model,
		Messages: []Message{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream: false,
	}

	b, _ := json.Marshal(reqBody)
	resp, err := client.Post(requestUrl, "application/json", bytes.NewReader(b))
	if err != nil {
		slog.Error("OllamaRequest error", "error", err)
		panic(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		slog.Error("OllamaRequest non-200 status", "status", resp.StatusCode, "body", string(body))
		panic(fmt.Sprintf("status=%d body=%s", resp.StatusCode, string(body)))
	}

	var out ChatResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		slog.Error("OllamaRequest decode error", "error", err)
		panic(err)
	}

	slog.Info("OllamaRequest success")

	return out.Message.Content, nil
}

func OllamaEmbedRequest(model string, requestURL string, text string) ([]float32, error) {
	slog.Info("Requesting Ollama embed", "model", model)
	client := &http.Client{Timeout: 1200 * time.Second}

	reqBody := ollamaEmbedReq{
		Model: model,
		Input: text,
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	resp, err := client.Post(requestURL, "application/json", bytes.NewReader(b))
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

	slog.Info("Ollama embed success")

	return vec32, nil
}
