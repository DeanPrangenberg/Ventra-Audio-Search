use crate::utils::api::client::ApiClient;
use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize)]
pub struct SearchRequest {
    pub ts_query: String,
    pub semantic_search_query: String,
    pub category: String,
    pub start_time_period_iso: String,
    pub end_time_period_iso: String,
    pub max_segment_return: u64,
}

#[derive(Debug, Deserialize)]
pub struct AudioData {
    pub audiofile_hash: String,
    pub title: String,
    pub recording_date: String,
    pub duration_in_sec: f32,
    pub transcript_full: String,
    pub user_summary: String,
    pub ai_keywords: Vec<String>,
    pub ai_summary: String,

    #[serde(default)]
    pub error: String,
}

#[derive(Debug, Deserialize)]
pub struct SegmentData {
    pub segment_hash: String,
    pub audiofile_hash: String,
    pub sentence_index: f32,
    pub transcript: String,

    #[serde(default)]
    pub ts_score: f64,

    #[serde(rename = "vector_score", default)]
    pub query_score: f32,

    #[serde(default)]
    pub error: String,
}

#[derive(Debug, Deserialize)]
pub struct SearchResponse {
    #[serde(rename = "full_audio_data", default)]
    pub full_audio_data: Vec<AudioData>,

    #[serde(default)]
    pub top_k_segments: Vec<SegmentData>,

    pub ok: bool,

    #[serde(default)]
    pub error: String,
}

impl ApiClient {
    pub async fn search(&self, req: SearchRequest) -> Result<SearchResponse, reqwest::Error> {
        self.client
            .post(&self.url)
            .json(&req)
            .send()
            .await?
            .error_for_status()?
            .json::<SearchResponse>()
            .await
    }
}
