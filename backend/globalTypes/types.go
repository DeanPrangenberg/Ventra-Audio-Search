package globalTypes

import (
	"context"
	"encoding/base64"
	"fmt"
)

type AudioDataElement struct {
	AudiofileHash            string           `json:"audiofile_hash"`
	Title                    string           `json:"title"`
	RecordingDate            string           `json:"recording_date"`
	Base64Data               string           `json:"base64_data"`
	FileUrl                  string           `json:"file_url"`
	Category                 string           `json:"category"`
	AudioType                string           `json:"audio_type"`
	DownloadPath             string           `json:"-"`
	DurationInSec            float32          `json:"duration_in_sec"`
	TranscriptFull           string           `json:"transcript_full"`
	UserSummary              string           `json:"user_summary"`
	AiKeywords               []string         `json:"ai_keywords"`
	AiSummary                string           `json:"ai_summary"`
	SegmentElements          []SegmentElement `json:"-"`
	FileSavedOnDisk          bool             `json:"-"`
	InitInsertedInDB         bool             `json:"-"`
	FullTranscriptInDB       bool             `json:"-"`
	AllSegmentsInDB          bool             `json:"-"`
	SegmentsEmbeddingCreated bool             `json:"-"`
	AISummaryInDB            bool             `json:"-"`
	AIKeywordsInDB           bool             `json:"-"`
	FullyCompleted           bool             `json:"-"`
	RetryCounter             int              `json:"-"`
}

type SegmentElement struct {
	SegmentHash             string    `json:"-"`
	AudiofileHash           string    `json:"audiofile_hash"`
	StartInSec              float32   `json:"start_in_sec"`
	EndInSec                float32   `json:"end_in_sec"`
	Transcript              string    `json:"transcript"`
	TranscriptEmbedding     []float32 `json:"-"`
	SegmentInDB             bool      `json:"-"`
	TranscriptEmbeddingDone bool      `json:"-"`
	BM25                    float64   `json:"bm25_score"`
	QueryScore              float32   `json:"vector_score"`
}

func (s *AudioDataElement) ValidateApiInput() error {
	if s.Title == "" {
		return fmt.Errorf("title is empty")
	}

	if s.UserSummary == "" {
		return fmt.Errorf("user_summary is empty")
	}

	if len(s.Base64Data) > 0 {
		_, err := base64.StdEncoding.DecodeString(s.Base64Data)
		if err != nil {
			return fmt.Errorf("invalid base64 in base64_data: %v", err)
		}

	} else if len(s.FileUrl) > 0 {
		_, code, err := urlIsDownloadable(context.Background(), s.FileUrl)
		if err != nil {
			return fmt.Errorf("error while checking file_url %s Status %d: %v", s.FileUrl, code, err)
		}
	} else {
		return fmt.Errorf("file_url and base64_data is empty")
	}

	return nil
}

func (s *AudioDataElement) ToString() string {
	return fmt.Sprint(
		"AudiofileHash: " + s.AudiofileHash + "\n" +
			"Title: " + s.Title + "\n" +
			"RecordingDate: " + s.RecordingDate + "\n" +
			"Base64Data: " + s.Base64Data + "\n" +
			"FileUrl: " + s.FileUrl + "\n" +
			"Category: " + s.Category + "\n" +
			"AudioType: " + s.AudioType + "\n" +
			"DownloadPath: " + s.DownloadPath + "\n" +
			"DurationInSec: " + fmt.Sprintf("%.2f", s.DurationInSec) + "\n" +
			"TranscriptFull: " + s.TranscriptFull + "\n" +
			"UserSummary: " + s.UserSummary + "\n" +
			"AiKeywords: " + fmt.Sprintf("%v", s.AiKeywords) + "\n" +
			"AiSummary: " + s.AiSummary,
	)
}
