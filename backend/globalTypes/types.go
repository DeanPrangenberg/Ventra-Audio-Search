package globalTypes

import (
	"context"
	"encoding/base64"
	"fmt"
	"go_audio_search_api_server/globalUtils"
)

// ProcessingStage represents the different stages of processing an audio file, from receiving the data to completing all processing stages
type ProcessingStage int

const (
	// StageQueued Init Stage saved needed data in db
	StageQueued ProcessingStage = 1
	// StageFilePersisted File saved to disk, ready for processing
	StageFilePersisted ProcessingStage = 2
	// StageTranscribed Created full transcript and segments and saved in postgres
	StageTranscribed ProcessingStage = 3
	// StageEmbedded Embeddings created for all segments and saved in qdrant
	StageEmbedded ProcessingStage = 4
	// StageAiDataGenerated Created summary and keywords
	StageAiDataGenerated ProcessingStage = 5
	// StageCompleted All stages completed successfully
	StageCompleted ProcessingStage = 0
	// StageFailed Failed in one of the stages
	StageFailed ProcessingStage = -1
)

// AudioDataElement represents the structure of the audio data received from the API and used throughout the processing pipeline
type AudioDataElement struct {
	AudiofileHash       string           `json:"audiofile_hash"`
	Title               string           `json:"title"`
	RecordingDate       string           `json:"recording_date"`
	Base64Data          string           `json:"base64_data"`
	FileUrl             string           `json:"file_url"`
	Category            string           `json:"category"`
	AudioType           string           `json:"audio_type"`
	DownloadPath        string           `json:"-"`
	DurationInSec       float32          `json:"duration_in_sec"`
	TranscriptFull      string           `json:"transcript_full"`
	UserSummary         string           `json:"user_summary"`
	AiKeywords          []string         `json:"ai_keywords"`
	AiSummary           string           `json:"ai_summary"`
	SegmentElements     []SegmentElement `json:"-"`
	LastSuccessfulStage ProcessingStage  `json:"last_successful_stage"`
	RetryCounter        int              `json:"retry_counter"`
}

// UpdateToNextStage updates the LastSuccessfulStage to the next stage in the processing pipeline
func (s *AudioDataElement) UpdateToNextStage() {
	switch s.LastSuccessfulStage {
	case StageQueued:
		s.LastSuccessfulStage = StageFilePersisted
	case StageFilePersisted:
		s.LastSuccessfulStage = StageTranscribed
	case StageTranscribed:
		s.LastSuccessfulStage = StageEmbedded
	case StageEmbedded:
		s.LastSuccessfulStage = StageAiDataGenerated
	default:
		s.LastSuccessfulStage = StageFailed
	}
}

// SegmentElement represents a segment of the audio file with its transcript and embedding information
type SegmentElement struct {
	SegmentHash             string    `json:"-"`
	AudiofileHash           string    `json:"audiofile_hash"`
	StartInSec              float32   `json:"start_in_sec"`
	EndInSec                float32   `json:"end_in_sec"`
	Transcript              string    `json:"transcript"`
	TranscriptEmbedding     []float32 `json:"-"`
	SegmentInDB             bool      `json:"-"`
	TranscriptEmbeddingDone bool      `json:"-"`
	TsScore                 float64   `json:"ts_score"`
	QueryScore              float32   `json:"vector_score"`
}

// ValidateApiInput validates the input data for the AudioDataElement
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

// ToString creates a string representation of the AudioDataElement
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
func (s *AudioDataElement) GetTmpHash() string {
	return "tmp_" + globalUtils.StringSha256Hex(s.ToString())
}
