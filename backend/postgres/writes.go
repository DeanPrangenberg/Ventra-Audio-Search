package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"go_audio_search_api_server/globalTypes"
)

// UpsertBase schreibt Metadaten in Postgres.
func (s *PostgressWrapper) UpsertBase(ctx context.Context, a *globalTypes.AudioDataElement) error {
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
  last_successful_step,
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
  download_path        = EXCLUDED.download_path,
  duration_in_sec      = EXCLUDED.duration_in_sec,
  last_successful_step = EXCLUDED.last_successful_step,
  retry_counter        = EXCLUDED.retry_counter,
  transcript_full      = COALESCE(EXCLUDED.transcript_full, audiofiles.transcript_full),
  user_summary_text    = COALESCE(EXCLUDED.user_summary_text, audiofiles.user_summary_text),
  ai_keywords          = COALESCE(EXCLUDED.ai_keywords, audiofiles.ai_keywords),
  ai_summary           = COALESCE(EXCLUDED.ai_summary, audiofiles.ai_summary);
`

	_, err = s.db.ExecContext(ctx, q,
		a.AudiofileHash,
		nullIfEmpty(a.Title),
		a.RecordingDate, // erwartet ISO: YYYY-MM-DD
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
		a.LastSuccessfulStep,
		a.RetryCounter,
	)
	return err
}

func (s *PostgressWrapper) UpdateAudiofileHash(ctx context.Context, oldAudioHash string, newAudioHash string) error {
	if oldAudioHash == "" {
		return errors.New("oldAudioHash required")
	}
	if newAudioHash == "" {
		return errors.New("newAudioHash required")
	}

	const q = `UPDATE audiofiles SET audiofile_hash = $1 WHERE audiofile_hash = $2;`
	_, err := s.db.ExecContext(ctx, q, newAudioHash, oldAudioHash)
	return err
}

func (s *PostgressWrapper) UpdateTranscriptFull(ctx context.Context, audioHash string, transcript string) error {
	if audioHash == "" {
		return errors.New("audioHash required")
	}

	const q = `UPDATE audiofiles SET transcript_full = $1 WHERE audiofile_hash = $2;`
	_, err := s.db.ExecContext(ctx, q, transcript, audioHash)
	return err
}

func (s *PostgressWrapper) UpdateUserSummaryText(ctx context.Context, audioHash string, summary string) error {
	if audioHash == "" {
		return errors.New("audioHash required")
	}

	const q = `UPDATE audiofiles SET user_summary_text = $1 WHERE audiofile_hash = $2;`
	_, err := s.db.ExecContext(ctx, q, summary, audioHash)
	return err
}

func (s *PostgressWrapper) UpdateAISummary(ctx context.Context, audioHash string, aiSummary string) error {
	if audioHash == "" {
		return errors.New("audioHash required")
	}

	const q = `UPDATE audiofiles SET ai_summary = $1 WHERE audiofile_hash = $2;`
	_, err := s.db.ExecContext(ctx, q, aiSummary, audioHash)
	return err
}

func (s *PostgressWrapper) UpdateAIKeywords(ctx context.Context, audioHash string, keywords []string) error {
	if audioHash == "" {
		return errors.New("audioHash required")
	}

	b, err := json.Marshal(keywords)
	if err != nil {
		return fmt.Errorf("marshal keywords: %w", err)
	}

	const q = `UPDATE audiofiles SET ai_keywords = $1::jsonb WHERE audiofile_hash = $2;`
	_, err = s.db.ExecContext(ctx, q, string(b), audioHash)
	return err
}

// InsertSegmentsUpsert schreibt Segmente als Batch in einer TX.
func (s *PostgressWrapper) InsertSegmentsUpsert(ctx context.Context, audioHash string, segs *[]globalTypes.SegmentElement) error {
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

func (s *PostgressWrapper) ResetProcessingClaims(ctx context.Context) error {
	const q = `
UPDATE audiofiles
SET gets_processed = FALSE
WHERE gets_processed = TRUE;
`
	_, err := s.db.ExecContext(ctx, q)
	return err
}
