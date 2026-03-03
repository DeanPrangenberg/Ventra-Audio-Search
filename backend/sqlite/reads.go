package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"go_audio_search_api_server/globalTypes"
	"strings"
)

func (s *SQLiteStore) GetSearchAudioDataByHash(ctx context.Context, audioHash string) (*globalTypes.SearchAudioData, error) {
	const q = `
SELECT audiofile_hash, title, recording_date, duration_in_sec,
       transcript_full, user_summary_text, ai_keywords, ai_summary
FROM audiofiles
WHERE audiofile_hash = ?;
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

func (s *SQLiteStore) GetAllSegmentsByAudioHash(ctx context.Context, audioHash string) (*[]globalTypes.SegmentElement, error) {
	const q = `
SELECT segment_hash, start_sec, end_sec, transcript
FROM segments
WHERE audiofile_hash = ?;
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
		err := rows.Scan(
			&segment.SegmentHash,
			&start, &end,
			&segment.Transcript,
		)

		if err != nil {
			return nil, err
		}
		segment.AudiofileHash = audioHash
		segment.StartInSec = float32(start)
		segment.EndInSec = float32(end)
		out = append(out, segment)
	}
	return &out, rows.Err()
}

func (s *SQLiteStore) ClaimNextAudioForProcessing(ctx context.Context) (*globalTypes.AudioDataElement, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	const q = `
UPDATE audiofiles
SET gets_processed = TRUE
WHERE audiofile_hash = (
    SELECT audiofile_hash
    FROM audiofiles
    WHERE last_successful_step > 0
      AND gets_processed = FALSE
    ORDER BY last_successful_step ASC, rowid ASC
    LIMIT 1
)
RETURNING audiofile_hash, 
          title, 
          recording_date, 
          category, 
          audio_type, 
          base64_data, 
          file_url, 
          download_path, 
          duration_in_sec, 
          transcript_full, 
          user_summary_text, 
          ai_keywords, 
          ai_summary, 
          last_successful_step`

	var r globalTypes.AudioDataElement
	var step int64

	err = tx.QueryRowContext(ctx, q).Scan(
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
		&r.AiKeywords,
		&r.AiSummary,
		&step,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	r.LastSuccessfulStep = globalTypes.ProcessingStage(step)

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &r, nil
}

func (s *SQLiteStore) GetSegmentByHash(ctx context.Context, segmentHash string) (*globalTypes.SearchSegmentData, error) {
	const q = `
SELECT segment_hash, audiofile_hash, start_sec, end_sec, transcript
FROM segments
WHERE segment_hash = ?;
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

func (s *SQLiteStore) FTS5Candidates(ctx context.Context, userInput string, k int, category string, startDateISO string, endDateISO string) ([]globalTypes.SegmentElement, error) {
	if strings.TrimSpace(userInput) == "" {
		return nil, errors.New("userInput empty")
	}

	ftsQuery := UserTextToFTS(userInput)

	if k <= 0 {
		k = 200
	}

	const q = `
SELECT
  s.segment_hash,
  s.audiofile_hash,
  s.start_sec,
  s.end_sec,
  s.transcript,
  bm25(segments_fts) AS score
FROM segments_fts
JOIN segments s  ON s.rowid = segments_fts.rowid
JOIN audiofiles a ON a.audiofile_hash = s.audiofile_hash
WHERE segments_fts MATCH ?
  AND a.recording_date >= ?
  AND a.recording_date < ?
  AND a.category IS ?
ORDER BY score
LIMIT ?;`
	rows, err := s.db.QueryContext(ctx, q, ftsQuery, startDateISO, endDateISO, category, k)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]globalTypes.SegmentElement, 0, k)
	for rows.Next() {
		var segment globalTypes.SegmentElement
		var start, end float64
		err := rows.Scan(
			&segment.SegmentHash,
			&segment.AudiofileHash,
			&start, &end,
			&segment.Transcript,
			&segment.BM25,
		)

		if err != nil {
			return nil, err
		}
		segment.StartInSec = float32(start)
		segment.EndInSec = float32(end)
		out = append(out, segment)
	}
	return out, rows.Err()
}
