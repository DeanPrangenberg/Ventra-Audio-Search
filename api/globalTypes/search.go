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
    SentenceIndex float32 `json:"sentence_index"`
    Transcript    string  `json:"transcript"`
    TsScore       float64 `json:"ts_score,omitempty"`
    QueryScore    float32 `json:"vector_score,omitempty"`
    Error         string  `json:"error,omitempty"`
}

type SearchRequest struct {
    TsQuery             string `json:"ts_query,omitempty"`
    SemanticSearchQuery string `json:"semantic_search_query,omitempty"`
    Category            string `json:"category"`
    StartTimePeriodIso  string `json:"start_time_period_iso"`
    EndTimePeriodIso    string `json:"end_time_period_iso"`
    MaxSegmentReturn    uint64 `json:"max_segment_return"`
}

type SearchResponse struct {
    RelatedAudioData []SearchAudioData   `json:"full_audio_data,omitempty"`
    TopKSegments     []SearchSegmentData `json:"top_k_segments,omitempty"`
    Ok               bool                `json:"ok"`
    Err              string              `json:"error,omitempty"`
}

func (s *SearchRequest) ValidateApiInput() error {
    if s.TsQuery == "" && s.SemanticSearchQuery == "" {
        return fmt.Errorf("ts_query and semantic_search_query are empty")
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
