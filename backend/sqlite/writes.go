package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go_audio_search_api_server/globalTypes"
)

// UpsertBase schreibt "Metadaten", ohne dass du Transcript/AI zwingend setzen musst.
func (s *SQLiteStore) UpsertBase(ctx context.Context, a *globalTypes.AudioDataElement) error {
	if a == nil {
		return errors.New("nil audio element")
	}
	if a.AudiofileHash == "" {
		return errors.New("AudiofileHash required")
	}

	kwJSON, err := json.Marshal(a.AiKeywords)
	if err != nil {
		return fmt.Errorf("marshal AIKeywords: %w", err)
	}

	const q = `
INSERT INTO audiofiles (
  audiofile_hash, title, recording_date, category, audio_type,base64_data, file_url, download_path, duration_in_sec,
  transcript_full, user_summary_text, ai_keywords_json, ai_summary
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(audiofile_hash) DO UPDATE SET
  title             = excluded.title,
  recording_date    = excluded.recording_date,
  category    		= excluded.category,
  audio_type    	= excluded.audio_type,
  base64_data       = excluded.base64_data,
  file_url          = excluded.file_url,
  download_path     = excluded.download_path,
  duration_in_sec   = excluded.duration_in_sec,
  transcript_full   = COALESCE(excluded.transcript_full, audiofiles.transcript_full),
  user_summary_text = COALESCE(excluded.user_summary_text, audiofiles.user_summary_text),
  ai_keywords_json  = COALESCE(excluded.ai_keywords_json, audiofiles.ai_keywords_json),
  ai_summary        = COALESCE(excluded.ai_summary, audiofiles.ai_summary);
`
	_, err = s.db.ExecContext(ctx, q,
		a.AudiofileHash,
		nullIfEmpty(a.Title),
		nullIfEmpty(a.RecordingDate),
		nullIfEmpty(a.Category),
		nullIfEmpty(a.AudioType),
		nullIfEmpty(a.Base64Data),
		nullIfEmpty(a.FileUrl),
		nullIfEmpty(a.DownloadPath),
		a.DurationInSec,
		nullIfEmpty(a.TranscriptFull),
		nullIfEmpty(a.UserSummary),
		string(kwJSON),
		nullIfEmpty(a.AiSummary),
	)
	return err
}

func (s *SQLiteStore) UpdateTranscriptFull(ctx context.Context, audioHash string, transcript string) error {
	if audioHash == "" {
		return errors.New("audioHash required")
	}
	const q = `UPDATE audiofiles SET transcript_full = ? WHERE audiofile_hash = ?;`
	_, err := s.db.ExecContext(ctx, q, transcript, audioHash)
	return err
}

func (s *SQLiteStore) UpdateUserSummaryText(ctx context.Context, audioHash string, summary string) error {
	if audioHash == "" {
		return errors.New("audioHash required")
	}
	const q = `UPDATE audiofiles SET user_summary_text = ? WHERE audiofile_hash = ?;`
	_, err := s.db.ExecContext(ctx, q, summary, audioHash)
	return err
}

func (s *SQLiteStore) UpdateAISummary(ctx context.Context, audioHash string, aiSummary string) error {
	if audioHash == "" {
		return errors.New("audioHash required")
	}
	const q = `UPDATE audiofiles SET ai_summary = ? WHERE audiofile_hash = ?;`
	_, err := s.db.ExecContext(ctx, q, aiSummary, audioHash)
	return err
}

func (s *SQLiteStore) UpdateAIKeywords(ctx context.Context, audioHash string, keywords []string) error {
	if audioHash == "" {
		return errors.New("audioHash required")
	}
	b, err := json.Marshal(keywords)
	if err != nil {
		return fmt.Errorf("marshal keywords: %w", err)
	}
	const q = `UPDATE audiofiles SET ai_keywords_json = ? WHERE audiofile_hash = ?;`
	_, err = s.db.ExecContext(ctx, q, string(b), audioHash)
	return err
}

// InsertSegmentsUpsert schreibt Segmente (ohne Embeddings). Batch in einer TX.
func (s *SQLiteStore) InsertSegmentsUpsert(ctx context.Context, audioHash string, segs []globalTypes.SegmentElement) error {
	if audioHash == "" {
		return errors.New("audioHash required")
	}
	if len(segs) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	const q = `
INSERT INTO segments (segment_hash, audiofile_hash, start_sec, end_sec, transcript)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(segment_hash) DO UPDATE SET
  audiofile_hash = excluded.audiofile_hash,
  start_sec      = excluded.start_sec,
  end_sec        = excluded.end_sec,
  transcript     = excluded.transcript;
`
	stmt, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, sgm := range segs {
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
