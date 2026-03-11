package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go_audio_search_api_server/globalUtils"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sync/semaphore"
)

type WhisperWorker struct {
	BaseURL   string
	Timeout   time.Duration
	Temp      string
	TempInc   string
	Format    string
	Language  string
	MinSegSec float32
	sem       *semaphore.Weighted
}

type Segment struct {
	SentenceIndex int
	Transcript    string `json:"text"`
}

type TranscriptionResult struct {
	Transcript string `json:"text"`
	Segments   []Segment
}

func New(minSegSec float32) *WhisperWorker {
	whisperReplicas := globalUtils.LoadEnvInt("WHISPER_REPLICAS")

	return &WhisperWorker{
		BaseURL:   globalUtils.LoadEnvStr("WHISPER_API_URL"),
		Timeout:   30 * time.Minute,
		Temp:      "0.0",
		TempInc:   "0.2",
		Format:    "json",
		Language:  "de",
		MinSegSec: minSegSec,
		sem:       semaphore.NewWeighted(int64(whisperReplicas)),
	}
}

func (wa *WhisperWorker) Transcribe(ctx context.Context, filePath string) (*TranscriptionResult, error) {
	if wa.BaseURL == "" {
		wa.BaseURL = "http://127.0.0.1:9001"
	}
	if wa.Timeout == 0 {
		wa.Timeout = 5 * time.Minute
	}

	raw, err := wa.transcribeRaw(ctx, filePath)
	if err != nil {
		return nil, err
	}

	var out TranscriptionResult
	if err := json.Unmarshal(raw, &out); err != nil {
		snippet := string(raw)
		if len(snippet) > 400 {
			snippet = snippet[:400] + "..."
		}
		return nil, fmt.Errorf("unmarshal whisper response failed: %w (snippet: %q)", err, snippet)
	}

	out.Segments, err = SplitSentences(out.Transcript, 3, 2)

	slog.Info("whisper transcription completed",
		"file", filePath,
		"transcript_len", len(out.Transcript),
		"segments", len(out.Segments),
	)

	return &out, nil
}

func (wa *WhisperWorker) transcribeRaw(ctx context.Context, filePath string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	part, err := w.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, fmt.Errorf("copy file to multipart: %w", err)
	}

	// exakt wie dein funktionierender code – nur kompakter
	if err := w.WriteField("temperature", wa.Temp); err != nil {
		return nil, fmt.Errorf("write field temperature: %w", err)
	}
	if err := w.WriteField("temperature_inc", wa.TempInc); err != nil {
		return nil, fmt.Errorf("write field temperature_inc: %w", err)
	}
	if err := w.WriteField("response_format", wa.Format); err != nil {
		return nil, fmt.Errorf("write field response_format: %w", err)
	}
	if err := w.WriteField("language", wa.Language); err != nil {
		return nil, fmt.Errorf("write field language: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	if err := wa.sem.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wa.BaseURL+"/inference", &body)
	wa.sem.Release(1)

	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	client := &http.Client{Timeout: wa.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("inference failed: %s: %s", resp.Status, string(respBody))
	}

	return respBody, nil
}

func SplitSentences(text string, chunkSize int, overlap int) ([]Segment, error) {
	if chunkSize <= 0 {
		return nil, fmt.Errorf("chunk size must be greater than zero")
	}

	if overlap < 0 {
		return nil, fmt.Errorf("overlap must not be negative")
	}

	if overlap >= chunkSize {
		return nil, fmt.Errorf("overlap must be smaller than chunk size")
	}

	re := regexp.MustCompile(`(?s).*?[.!?](?:\s+|$)`)
	sentences := re.FindAllString(text, -1)

	var cleanedSentences []string
	for _, sentence := range sentences {
		cleaned := strings.TrimSpace(sentence)
		cleaned = strings.ReplaceAll(cleaned, "\n", " ")
		if cleaned != "" {
			cleanedSentences = append(cleanedSentences, cleaned)
		}
	}

	if len(cleanedSentences) == 0 {
		return nil, nil
	}

	step := chunkSize - overlap
	var segments []Segment

	for start := 0; start < len(cleanedSentences); start += step {
		end := start + chunkSize
		if end > len(cleanedSentences) {
			end = len(cleanedSentences)
		}

		chunk := strings.Join(cleanedSentences[start:end], " ")

		segments = append(segments, Segment{
			SentenceIndex: start,
			Transcript:    chunk,
		})

		if end == len(cleanedSentences) {
			break
		}
	}

	return segments, nil
}
