package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"go_audio_search_api_server/globalTypes"
)

func (s *Worker) GetSearchAudioDataByHash(ctx context.Context, audioHash string) (*globalTypes.SearchAudioData, error) {
	const q = `
SELECT
  audiofile_hash,
  COALESCE(title, ''),
  COALESCE(recording_date::text, ''),
  COALESCE(duration_in_sec, 0),
  COALESCE(transcript_full, ''),
  COALESCE(user_summary_text, ''),
  COALESCE(ai_keywords::text, ''),
  COALESCE(ai_summary, '')
FROM audiofiles
WHERE audiofile_hash = $1;
`

	var r globalTypes.SearchAudioData
	err := s.db.QueryRowContext(ctx, q, audioHash).Scan(
		&r.AudiofileHash,
		&r.Title,
		&r.RecordingDate,
		&r.DurationInSec,
		&r.TranscriptFull,
		&r.UserSummary,
		&r.AiKeywords,
		&r.AiSummary,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &r, nil
}

func (s *Worker) GetAllSegmentsByAudioHash(ctx context.Context, audioHash string) ([]globalTypes.SegmentElement, error) {
	const q = `
SELECT segment_hash, start_sec, end_sec, transcript
FROM segments
WHERE audiofile_hash = $1
ORDER BY start_sec ASC;
`

	rows, err := s.db.QueryContext(ctx, q, audioHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []globalTypes.SegmentElement
	for rows.Next() {
		var segment globalTypes.SegmentElement
		var start, end float64

		if err := rows.Scan(
			&segment.SegmentHash,
			&start,
			&end,
			&segment.Transcript,
		); err != nil {
			return nil, err
		}

		segment.AudiofileHash = audioHash
		segment.StartInSec = float32(start)
		segment.EndInSec = float32(end)
		out = append(out, segment)
	}

	return out, rows.Err()
}

func (s *Worker) ClaimNextAudioForProcessing(
	ctx context.Context,
	lastSuccessfulStage globalTypes.ProcessingStage,
	amount uint64,
) ([]*globalTypes.AudioDataElement, error) {
	if amount == 0 {
		return nil, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	const q = `
WITH next_rows AS (
    SELECT audiofile_hash
    FROM audiofiles
    WHERE last_successful_stage = $1
      AND gets_processed = FALSE
    ORDER BY last_successful_stage ASC, created_at ASC
    FOR UPDATE SKIP LOCKED
    LIMIT $2
)
UPDATE audiofiles a
SET gets_processed = TRUE
FROM next_rows
WHERE a.audiofile_hash = next_rows.audiofile_hash
RETURNING
  a.audiofile_hash,
  COALESCE(a.title, ''),
  COALESCE(a.recording_date::text, ''),
  COALESCE(a.category, ''),
  COALESCE(a.audio_type, ''),
  COALESCE(a.base64_data, ''),
  COALESCE(a.file_url, ''),
  COALESCE(a.download_path, ''),
  COALESCE(a.duration_in_sec, 0),
  COALESCE(a.transcript_full, ''),
  COALESCE(a.user_summary_text, ''),
  COALESCE(a.ai_keywords::text, '[]'),
  COALESCE(a.ai_summary, ''),
  COALESCE(a.last_successful_stage, 0),
  COALESCE(a.retry_counter, 0);
`

	rows, err := tx.QueryContext(ctx, q, int64(lastSuccessfulStage), int64(amount))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*globalTypes.AudioDataElement, 0, minInt(int(amount), 128))

	for rows.Next() {
		var r globalTypes.AudioDataElement
		var stage int64
		var aiKeywordsJSON string

		if err := rows.Scan(
			&r.AudiofileHash,
			&r.Title,
			&r.RecordingDate,
			&r.Category,
			&r.AudioType,
			&r.Base64Data,
			&r.FileUrl,
			&r.DownloadPath,
			&r.DurationInSec,
			&r.TranscriptFull,
			&r.UserSummary,
			&aiKeywordsJSON,
			&r.AiSummary,
			&stage,
			&r.RetryCounter,
		); err != nil {
			return nil, err
		}

		r.AiKeywords = stringSliceFromJSON(aiKeywordsJSON)
		r.LastSuccessfulStage = globalTypes.ProcessingStage(stage)

		out = append(out, &r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func (s *Worker) GetSegmentByHash(ctx context.Context, segmentHash string) (*globalTypes.SearchSegmentData, error) {
	const q = `
SELECT segment_hash, audiofile_hash, start_sec, end_sec, transcript
FROM segments
WHERE segment_hash = $1;
`

	var r globalTypes.SearchSegmentData
	var start, end float64

	err := s.db.QueryRowContext(ctx, q, segmentHash).Scan(
		&r.SegmentHash,
		&r.AudiofileHash,
		&start,
		&end,
		&r.Transcript,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	r.StartInSec = float32(start)
	r.EndInSec = float32(end)

	return &r, nil
}

// GetPostgresCandidates bleibt absichtlich gleich benannt, damit dein Restcode nicht bricht.
// Intern ist das jetzt Postgres Full Text Search.
func (s *Worker) GetPostgresCandidates(ctx context.Context, userInput string, k int, category string, startDateISO string, endDateISO string) ([]globalTypes.SegmentElement, error) {
	if strings.TrimSpace(userInput) == "" {
		return nil, errors.New("userInput empty")
	}
	if k <= 0 {
		k = 200
	}

	const q = `
WITH search_query AS (
  SELECT websearch_to_tsquery('simple', $1) AS query
)
SELECT
  s.segment_hash,
  s.audiofile_hash,
  s.start_sec,
  s.end_sec,
  s.transcript,
  ts_rank_cd(s.transcript_tsv, search_query.query, 2) AS score
FROM segments s
JOIN audiofiles a ON a.audiofile_hash = s.audiofile_hash
CROSS JOIN search_query
WHERE s.transcript_tsv @@ search_query.query
  AND a.recording_date >= COALESCE(NULLIF($2, '')::date, DATE '0001-01-01')
  AND a.recording_date <  COALESCE(NULLIF($3, '')::date, DATE '9999-12-31')
  AND a.category IS NOT DISTINCT FROM $4
ORDER BY score DESC, s.start_sec ASC
LIMIT $5;
`

	rows, err := s.db.QueryContext(ctx, q, userInput, startDateISO, endDateISO, category, k)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]globalTypes.SegmentElement, 0, k)
	for rows.Next() {
		var segment globalTypes.SegmentElement
		var start, end float64

		if err := rows.Scan(
			&segment.SegmentHash,
			&segment.AudiofileHash,
			&start,
			&end,
			&segment.Transcript,
			&segment.TsScore, // Feldname bleibt, Inhalt ist jetzt Postgres-Rank
		); err != nil {
			return nil, err
		}

		segment.StartInSec = float32(start)
		segment.EndInSec = float32(end)
		out = append(out, segment)
	}

	return out, rows.Err()
}
