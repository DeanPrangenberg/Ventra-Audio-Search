package sqlite

import (
	"context"

	_ "github.com/mattn/go-sqlite3"
)

func (s *SQLiteStore) CreateTables(ctx context.Context) error {
	const ddl = `
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS audiofiles (
  audiofile_hash     TEXT PRIMARY KEY,
  title              TEXT,
  recording_date     TEXT,
  audio_type     	 TEXT,
  category           TEXT,
  base64_data        TEXT,
  file_url           TEXT,
  download_path      TEXT,
  duration_in_sec    REAL,
  transcript_full    TEXT,
  user_summary_text  TEXT,
  ai_keywords_json   TEXT,
  ai_summary         TEXT,
  created_at         TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at         TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TRIGGER IF NOT EXISTS audiofiles_updated_at
AFTER UPDATE ON audiofiles
BEGIN
  UPDATE audiofiles SET updated_at = datetime('now') WHERE audiofile_hash = NEW.audiofile_hash;
END;

CREATE TABLE IF NOT EXISTS segments (
  segment_hash    TEXT PRIMARY KEY,
  audiofile_hash  TEXT NOT NULL,
  start_sec       REAL NOT NULL,
  end_sec         REAL NOT NULL,
  transcript      TEXT NOT NULL,
  created_at      TEXT NOT NULL DEFAULT (datetime('now')),
  FOREIGN KEY (audiofile_hash) REFERENCES audiofiles(audiofile_hash) ON DELETE CASCADE,
  UNIQUE(audiofile_hash, start_sec, end_sec)
);

CREATE INDEX IF NOT EXISTS idx_segments_audiofile ON segments(audiofile_hash);
CREATE INDEX IF NOT EXISTS idx_audiofiles_recording_date ON audiofiles(recording_date);

CREATE VIRTUAL TABLE IF NOT EXISTS segments_fts USING fts5(
  transcript,
  segment_hash UNINDEXED,
  audiofile_hash UNINDEXED,
  start_sec UNINDEXED,
  end_sec UNINDEXED
);
       
CREATE TRIGGER IF NOT EXISTS segments_ai AFTER INSERT ON segments BEGIN
  INSERT INTO segments_fts(rowid, transcript, segment_hash, audiofile_hash, start_sec, end_sec)
  VALUES (new.rowid, new.transcript, new.segment_hash, new.audiofile_hash, new.start_sec, new.end_sec);
END;

CREATE TRIGGER IF NOT EXISTS segments_ad AFTER DELETE ON segments BEGIN
  DELETE FROM segments_fts WHERE rowid = old.rowid;
END;

CREATE TRIGGER IF NOT EXISTS segments_au AFTER UPDATE ON segments BEGIN
  UPDATE segments_fts
  SET transcript = new.transcript,
      segment_hash = new.segment_hash,
      audiofile_hash = new.audiofile_hash,
      start_sec = new.start_sec,
      end_sec = new.end_sec
  WHERE rowid = old.rowid;
END;
`
	_, err := s.db.ExecContext(ctx, ddl)
	return err
}
