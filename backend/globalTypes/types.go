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

type SearchRequest struct {
	Fts5Query           string `json:"fts5_query"`
	SemanticSearchQuery string `json:"semantic_search_query"`
	Category            string `json:"category"`
	StartTimePeriodIso  string `json:"start_time_period_iso"`
	EndTimePeriodIso    string `json:"end_time_period_iso"`
	MaxSegmentReturn    string `json:"max_segment_return"`
	BackendResponseChan chan SearchResponse
}

type SearchResponse struct {
	RelatedAudioData []AudioDataElement `json:"full_audio_data,omitempty"`
	TopKSegments     []SegmentElement   `json:"top_k_segments,omitempty"`
	Ok               bool               `json:"ok"`
	Err              string             `json:"error,omitempty"`
}

func (s *SearchRequest) ValidateApiInput() error {
	if s.Fts5Query == "" {
		return fmt.Errorf("fts5_query is empty")
	}

	if s.SemanticSearchQuery == "" {
		return fmt.Errorf("semantic_search_query is empty")
	}

	if s.Category == "" {
		return fmt.Errorf("category is empty")
	}

	if s.StartTimePeriodIso == "" {
		return fmt.Errorf("start_time_period_iso is empty")
	}

	if s.EndTimePeriodIso == "" {
		return fmt.Errorf("end_time_period_iso is empty")
	}

	if s.MaxSegmentReturn == "" {
		return fmt.Errorf("max_segment_return is empty")
	}

	return nil
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
