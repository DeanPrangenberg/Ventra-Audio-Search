package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"go_audio_search_api_server/globalTypes"
)

func (s *Worker) UpsertBase(ctx context.Context, a *globalTypes.AudioDataElement) error {
	if a == nil {
		return errors.New("nil audio element")
	}
	if a.AudiofileHash == "" {
		return errors.New("AudiofileHash required")
	}

	aiKeywordsJSON, err := jsonOrNilFromStringSlice(a.AiKeywords)
	if err != nil {
		return fmt.Errorf("marshal ai keywords: %w", err)
	}

	const q = `
INSERT INTO audiofiles (
  audiofile_hash,
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
  last_successful_stage,
  retry_counter
) VALUES (
  $1,
  $2,
  NULLIF($3, '')::date,
  $4,
  $5,
  $6,
  $7,
  $8,
  $9,
  $10,
  $11,
  $12::jsonb,
  $13,
  $14,
  $15
)
ON CONFLICT(audiofile_hash) DO UPDATE SET
  title                = EXCLUDED.title,
  recording_date       = EXCLUDED.recording_date,
  category             = EXCLUDED.category,
  audio_type           = EXCLUDED.audio_type,
  base64_data          = EXCLUDED.base64_data,
  file_url             = EXCLUDED.file_url,
  duration_in_sec      = EXCLUDED.duration_in_sec,
  last_successful_stage = EXCLUDED.last_successful_stage,
  retry_counter        = EXCLUDED.retry_counter,
  transcript_full      = COALESCE(EXCLUDED.transcript_full, audiofiles.transcript_full),
  download_path        = COALESCE(EXCLUDED.download_path, audiofiles.download_path),
  user_summary_text    = COALESCE(EXCLUDED.user_summary_text, audiofiles.user_summary_text),
  ai_keywords          = COALESCE(EXCLUDED.ai_keywords, audiofiles.ai_keywords),
  ai_summary           = COALESCE(EXCLUDED.ai_summary, audiofiles.ai_summary);
`

	_, err = s.db.ExecContext(ctx, q,
		a.AudiofileHash,
		nullIfEmpty(a.Title),
		a.RecordingDate,
		nullIfEmpty(a.Category),
		nullIfEmpty(a.AudioType),
		nullIfEmpty(a.Base64Data),
		nullIfEmpty(a.FileUrl),
		nullIfEmpty(a.DownloadPath),
		a.DurationInSec,
		nullIfEmpty(a.TranscriptFull),
		nullIfEmpty(a.UserSummary),
		aiKeywordsJSON,
		nullIfEmpty(a.AiSummary),
		a.LastSuccessfulStage,
		a.RetryCounter,
	)
	return err
}

func (s *Worker) UpsertBaseBatch(ctx context.Context, items []*globalTypes.AudioDataElement) error {
	if len(items) == 0 {
		return nil
	}

	const colsPerRow = 15
	const chunkSize = 1000

	const head = `
INSERT INTO audiofiles (
  audiofile_hash,
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
  last_successful_stage,
  retry_counter
) VALUES
`

	const tail = `
ON CONFLICT(audiofile_hash) DO UPDATE SET
  title                = EXCLUDED.title,
  recording_date       = EXCLUDED.recording_date,
  category             = EXCLUDED.category,
  audio_type           = EXCLUDED.audio_type,
  base64_data          = EXCLUDED.base64_data,
  file_url             = EXCLUDED.file_url,
  duration_in_sec      = EXCLUDED.duration_in_sec,
  last_successful_stage = EXCLUDED.last_successful_stage,
  retry_counter        = EXCLUDED.retry_counter,
  download_path        = COALESCE(EXCLUDED.download_path, audiofiles.download_path),
  transcript_full      = COALESCE(EXCLUDED.transcript_full, audiofiles.transcript_full),
  user_summary_text    = COALESCE(EXCLUDED.user_summary_text, audiofiles.user_summary_text),
  ai_keywords          = COALESCE(EXCLUDED.ai_keywords, audiofiles.ai_keywords),
  ai_summary           = COALESCE(EXCLUDED.ai_summary, audiofiles.ai_summary);
`

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for start := 0; start < len(items); start += chunkSize {
		end := start + chunkSize
		if end > len(items) {
			end = len(items)
		}

		batch := items[start:end]

		if len(batch)*colsPerRow > 65000 {
			return fmt.Errorf("batch too large: %d rows", len(batch))
		}

		var sb strings.Builder
		sb.Grow(len(head) + len(tail) + len(batch)*256)
		sb.WriteString(head)

		args := make([]any, 0, len(batch)*colsPerRow)

		for i, a := range batch {
			if a == nil {
				return fmt.Errorf("nil audio element at index %d", start+i)
			}
			if a.AudiofileHash == "" {
				return fmt.Errorf("AudiofileHash required at index %d", start+i)
			}

			aiKeywordsJSON, jerr := jsonOrNilFromStringSlice(a.AiKeywords)
			if jerr != nil {
				return fmt.Errorf("marshal ai keywords at index %d: %w", start+i, jerr)
			}

			off := i*colsPerRow + 1

			if i > 0 {
				sb.WriteString(",\n")
			}

			fmt.Fprintf(&sb,
				`($%d, $%d, NULLIF($%d, '')::date, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d::jsonb, $%d, $%d, $%d)`,
				off+0,
				off+1,
				off+2,
				off+3,
				off+4,
				off+5,
				off+6,
				off+7,
				off+8,
				off+9,
				off+10,
				off+11,
				off+12,
				off+13,
				off+14,
			)

			args = append(args,
				a.AudiofileHash,
				nullIfEmpty(a.Title),
				a.RecordingDate,
				nullIfEmpty(a.Category),
				nullIfEmpty(a.AudioType),
				nullIfEmpty(a.Base64Data),
				nullIfEmpty(a.FileUrl),
				nullIfEmpty(a.DownloadPath),
				a.DurationInSec,
				nullIfEmpty(a.TranscriptFull),
				nullIfEmpty(a.UserSummary),
				aiKeywordsJSON,
				nullIfEmpty(a.AiSummary),
				a.LastSuccessfulStage,
				a.RetryCounter,
			)
		}

		sb.WriteString(tail)

		if _, err := tx.ExecContext(ctx, sb.String(), args...); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Worker) UpdateAudiofileHash(ctx context.Context, oldAudioHash string, newAudioHash string) error {
	if oldAudioHash == "" {
		return errors.New("oldAudioHash required")
	}
	if newAudioHash == "" {
		return errors.New("newAudioHash required")
	}
	if oldAudioHash == newAudioHash {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	const q = `
UPDATE audiofiles
SET audiofile_hash = $1
WHERE audiofile_hash = $2;
`

	res, err := tx.ExecContext(ctx, q, newAudioHash, oldAudioHash)
	if err != nil {
		return fmt.Errorf("update audiofile hash: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	if rows > 1 {
		return fmt.Errorf("updated %d rows, expected exactly 1", rows)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

// UpsertSegments schreibt Segmente als Batch in einer TX.
func (s *Worker) UpsertSegments(ctx context.Context, audioHash string, segs *[]globalTypes.SegmentElement) error {
	if audioHash == "" {
		return errors.New("audioHash required")
	}
	if segs == nil || len(*segs) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	const q = `
INSERT INTO segments (segment_hash, audiofile_hash, start_sec, end_sec, transcript)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT(segment_hash) DO UPDATE SET
  audiofile_hash = EXCLUDED.audiofile_hash,
  start_sec      = EXCLUDED.start_sec,
  end_sec        = EXCLUDED.end_sec,
  transcript     = EXCLUDED.transcript;
`

	stmt, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, sgm := range *segs {
		if sgm.SegmentHash == "" {
			return errors.New("segment_hash required")
		}
		if sgm.AudiofileHash == "" {
			sgm.AudiofileHash = audioHash
		}

		if _, err := stmt.ExecContext(ctx,
			sgm.SegmentHash,
			sgm.AudiofileHash,
			sgm.StartInSec,
			sgm.EndInSec,
			sgm.Transcript,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Worker) ResetProcessingClaims(ctx context.Context) error {
	const q = `
UPDATE audiofiles
SET gets_processed = FALSE
WHERE gets_processed = TRUE;
`
	_, err := s.db.ExecContext(ctx, q)
	return err
}
