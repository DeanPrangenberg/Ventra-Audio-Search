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
	"strings"
	"sync"
	"time"

	"github.com/clipperhouse/uax29/sentences"
)

type WhisperWorker struct {
	BaseURL   string
	Timeout   time.Duration
	Temp      string
	TempInc   string
	Format    string
	Language  string
	MinSegSec float32
	lock      sync.Mutex
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
	return &WhisperWorker{
		BaseURL:   globalUtils.LoadEnvStr("WHISPER_API_URL"),
		Timeout:   30 * time.Minute,
		Temp:      "0.0",
		TempInc:   "0.2",
		Format:    "json",
		Language:  "de",
		MinSegSec: minSegSec,
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

	out.Segments, err = SplitSentences(out.Transcript)

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

	wa.lock.Lock()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wa.BaseURL+"/inference", &body)
	wa.lock.Unlock()
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

func SplitSentences(text string) ([]Segment, error) {
	seg := sentences.NewStringSegmenter(text)

	var out []Segment
	var idx int
	for seg.Next() {
		s := strings.TrimSpace(seg.Text())
		if s != "" {

			out = append(out,
				Segment{
					SentenceIndex: idx,
					Transcript:    s,
				})
		}
	}

	if err := seg.Err(); err != nil {
		return nil, err
	}

	return out, nil
}
