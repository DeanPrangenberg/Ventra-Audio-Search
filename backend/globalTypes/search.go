package globalTypes

import "fmt"

type SearchAudioData struct {
	AudiofileHash  string   `json:"audiofile_hash"`
	Title          string   `json:"title"`
	RecordingDate  string   `json:"recording_date"`
	DurationInSec  float32  `json:"duration_in_sec"`
	TranscriptFull string   `json:"transcript_full"`
	UserSummary    string   `json:"user_summary"`
	AiKeywords     []string `json:"ai_keywords"`
	AiSummary      string   `json:"ai_summary"`
	Error          string   `json:"error,omitempty"`
}

type SearchSegmentData struct {
	SegmentHash   string  `json:"segment_hash"`
	AudiofileHash string  `json:"audiofile_hash"`
	StartInSec    float32 `json:"start_in_sec"`
	EndInSec      float32 `json:"end_in_sec"`
	Transcript    string  `json:"transcript"`
	BM25          float64 `json:"bm25_score"`
	QueryScore    float32 `json:"vector_score"`
	Error         string  `json:"error,omitempty"`
}

type SearchRequest struct {
	Fts5Query           string `json:"fts5_query"`
	SemanticSearchQuery string `json:"semantic_search_query"`
	Category            string `json:"category"`
	StartTimePeriodIso  string `json:"start_time_period_iso"`
	EndTimePeriodIso    string `json:"end_time_period_iso"`
	MaxSegmentReturn    uint64 `json:"max_segment_return"`
	BackendResponseChan chan SearchResponse
}

type SearchResponse struct {
	RelatedAudioData []SearchAudioData   `json:"full_audio_data,omitempty"`
	TopKSegments     []SearchSegmentData `json:"top_k_segments,omitempty"`
	Ok               bool                `json:"ok"`
	Err              string              `json:"error,omitempty"`
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

	return nil
}
