package ai

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"go_audio_search_api_server/globalUtils"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

type LlmWorker struct {
	model      string
	requestUrl string
	lock       sync.Mutex
}

func NewLlmWorker() *LlmWorker {
	slog.Info("Creating new llmWorker...")
	model := globalUtils.LoadEnvStr("LLM_MODEL")
	ollama := globalUtils.LoadEnvStr("OLLAMA_API_URL")

	return &LlmWorker{
		model:      model,
		requestUrl: ollama + "/api/chat",
	}
}

func (w *LlmWorker) Summary(audioType string, input string) (string, error) {
	_, summarySysPrompt := w.getSysPrompts(audioType)
	return w.ollamaRequest(summarySysPrompt, input)
}

func (w *LlmWorker) Keywords(audioType string, input string) ([]string, error) {
	keywordSysPrompt, _ := w.getSysPrompts(audioType)

	req, err := w.ollamaRequest(keywordSysPrompt, input)
	if err != nil {
		return nil, err
	}

	r := csv.NewReader(strings.NewReader(req))
	keywords, err := r.Read()
	if err != nil {
		return nil, err
	}

	for i := range keywords {
		keywords[i] = strings.TrimSpace(keywords[i])
	}

	return keywords, nil
}

func (w *LlmWorker) ollamaRequest(sysPrompt string, userPrompt string) (string, error) {
	slog.Info("Requesting Ollama model", "model", w.model)

	client := &http.Client{Timeout: 1200 * time.Second}

	reqBody := ChatReq{
		Model: w.model,
		Messages: []Message{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream: false,
	}

	b, _ := json.Marshal(reqBody)

	w.lock.Lock()
	resp, err := client.Post(w.requestUrl, "application/json", bytes.NewReader(b))
	w.lock.Unlock()

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

func (w *LlmWorker) getSysPrompts(taskType string) (string, string) {
	switch taskType {
	case "Meeting":
		return meetingKeywordSysPrompt, meetingSummarySysPrompt
	case "Media":
		return mediaKeywordsSysPrompt, mediaSummarySysPrompt
	default:
		return genericKeywordsSysPrompt, genericSummarySysPrompt
	}
}

const meetingKeywordSysPrompt = "You will receive a transcript in any language. Output exactly one CSV line with no header or explanation: 5–20 short categories/keywords (nouns/terms only), comma-separated, in the same language as the input. No quotes, no trailing period, no duplicates."

const meetingSummarySysPrompt = "You will receive a transcript in any language. Reply in the same language as the input with: (1) a short plain-text paragraph at the top (3–6 sentences) summarizing it, and (2) bullet points containing only verifiable facts from the transcript (who/what/when/where/how much/decisions/outcomes/next steps). No speculation, no new info, no opinions. If something is not clear, label it as 'Unclear:' instead of guessing."

const mediaKeywordsSysPrompt = "You will receive a podcast/video transcript in any language. Output exactly one CSV line with no header or explanation: 8–25 broad topics/words (2–4 word phrases allowed), comma-separated, in the same language as the input. No quotes, no duplicates, no trailing period."

const mediaSummarySysPrompt = "You will receive a podcast/video transcript in any language. Reply in the same language as the input with: (1) a short plain-text paragraph at the top (3–6 sentences) describing the core content/thesis/storyline, and (2) bullet points with only claims that are supported by the transcript (key points, arguments, examples, conclusions, important numbers/names). No speculation, no opinions, do not add anything. Mark unclear items as 'Unclear:'."

const genericKeywordsSysPrompt = "You will receive a transcript describing what happens in an audio recording (actions/sounds/events) in any language. Output exactly one CSV line with no header or explanation: 8–25 broad keywords/topics (e.g., mentioned words, sounds, events, places/objects), comma-separated, in the same language as the input. No quotes, no duplicates."

const genericSummarySysPrompt = "You will receive a transcript describing what happens in an audio recording (actions/sounds/events) in any language. Reply in the same language as the input with: (1) a short plain-text paragraph at the top (2–5 sentences) describing what broadly happens, and (2) bullet points with content-focused facts from the transcript (sequence of events, who speaks/acts, relevant sounds, key statements, place/time hints if mentioned). No interpretation, no speculation, do not invent anything. If something is not clear, label it as 'Unclear:'."
