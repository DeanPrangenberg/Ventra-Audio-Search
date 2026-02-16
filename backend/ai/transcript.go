package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type WhisperApi struct {
	BaseURL   string
	Timeout   time.Duration
	Temp      string
	TempInc   string
	Format    string
	Language  string
	MinSegSec float32 // 0 = kein mergen
}

type Segment struct {
	ID         int     `json:"id"`
	Start      float32 `json:"start"`
	End        float32 `json:"end"`
	Transcript string  `json:"text"`
}

type TranscriptionResult struct {
	Transcript string    `json:"text"`
	Segments   []Segment `json:"segments"`
}

func NewWhisperApi(minSegSec float32) *WhisperApi {
	return &WhisperApi{
		BaseURL:   loadEnv("WHISPER_API_URL"), // falls du das schon hast
		Timeout:   30 * time.Minute,
		Temp:      "0.0",
		TempInc:   "0.2",
		Format:    "verbose_json",
		Language:  "de",
		MinSegSec: minSegSec,
	}
}

func (wa *WhisperApi) Transcribe(ctx context.Context, filePath string) (*TranscriptionResult, error) {
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

	if len(out.Segments) == 0 {
		return nil, fmt.Errorf("whisper returned no segments (raw: %s)", string(raw))
	}

	out.Segments = wa.mergeSegmentsMinLen(out.Segments)
	// optional: transcript aus segments neu bauen (falls server text leer lässt)
	if out.Transcript == "" {
		out.Transcript = joinSegments(out.Segments)
	}

	slog.Info("whisper transcription completed",
		"file", filePath,
		"transcript_len", len(out.Transcript),
		"segments", len(out.Segments),
	)

	return &out, nil
}

func (wa *WhisperApi) transcribeRaw(ctx context.Context, filePath string) ([]byte, error) {
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wa.BaseURL+"/inference", &body)
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

// MinSegSec = Mindest-Dauer pro Segment (End-Start).
// 0 => keine Änderung, nur IDs neu vergeben.
func (wa *WhisperApi) mergeSegmentsMinLen(in []Segment) []Segment {
	out := make([]Segment, 0, len(in))
	if len(in) == 0 {
		return out
	}

	if wa.MinSegSec <= 0 {
		for i := range in {
			in[i].ID = i
			out = append(out, in[i])
		}
		return out
	}

	id := 0
	i := 0
	for i < len(in) {
		seg := in[i]
		i++

		// solange Segment-Dauer zu kurz: an nächste dranhängen
		for (seg.End-seg.Start) < wa.MinSegSec && i < len(in) {
			next := in[i]
			i++

			// End erweitern
			if next.End > seg.End {
				seg.End = next.End
			}
			// Text anfügen (mit Leerzeichen)
			if seg.Transcript != "" && next.Transcript != "" {
				seg.Transcript += " "
			}
			seg.Transcript += next.Transcript
		}

		seg.ID = id
		id++
		out = append(out, seg)
	}

	return out
}

func joinSegments(segs []Segment) string {
	var b bytes.Buffer
	for _, s := range segs {
		if s.Transcript == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(s.Transcript)
	}
	return b.String()
}
