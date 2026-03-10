package postgres

import "context"

func (s *Worker) CreateTables(ctx context.Context) error {
	stmts := []string{
		`
CREATE TABLE IF NOT EXISTS audiofiles (
  audiofile_hash        text PRIMARY KEY,
  title                 text,
  recording_date        date,
  audio_type            text,
  category              text,
  base64_data           text,
  file_url              text,
  download_path         text,
  duration_in_sec       double precision,
  transcript_full       text,
  user_summary_text     text,
  ai_keywords           jsonb,
  ai_summary            text,
  last_successful_stage  integer,
  retry_counter         integer NOT NULL DEFAULT 0,
  gets_processed        boolean NOT NULL DEFAULT false,
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now()
);`,
		`
CREATE OR REPLACE FUNCTION set_audiofiles_updated_at()
RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;`,
		`DROP TRIGGER IF EXISTS audiofiles_updated_at ON audiofiles;`,
		`
CREATE TRIGGER audiofiles_updated_at
BEFORE UPDATE ON audiofiles
FOR EACH ROW
EXECUTE FUNCTION set_audiofiles_updated_at();`,
		`
CREATE TABLE IF NOT EXISTS segments (
  segment_hash    text PRIMARY KEY,
  audiofile_hash  text NOT NULL,
  sentence_index       integer NOT NULL,
  transcript      text NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  transcript_tsv  tsvector GENERATED ALWAYS AS (
    to_tsvector('simple', coalesce(transcript, ''))
  ) STORED,
  CONSTRAINT fk_segments_audiofile
    FOREIGN KEY (audiofile_hash)
    REFERENCES audiofiles(audiofile_hash)
    ON DELETE CASCADE
    ON UPDATE CASCADE,
  CONSTRAINT uq_segments_audio_range UNIQUE(audiofile_hash, sentence_index)
);`,
		`CREATE INDEX IF NOT EXISTS idx_segments_audiofile ON segments(audiofile_hash);`,
		`CREATE INDEX IF NOT EXISTS idx_audiofiles_recording_date ON audiofiles(recording_date);`,
		`CREATE INDEX IF NOT EXISTS idx_audiofiles_category ON audiofiles(category);`,
		`CREATE INDEX IF NOT EXISTS idx_segments_tsv ON segments USING GIN (transcript_tsv);`,
		`
CREATE INDEX IF NOT EXISTS idx_audiofiles_claim_queue
ON audiofiles (gets_processed, last_successful_stage, created_at)
WHERE gets_processed = false;`, `CREATE TABLE IF NOT EXISTS counters (
  counter_name  text PRIMARY KEY,
  counter_value bigint NOT NULL DEFAULT 0,
  updated_at    timestamptz NOT NULL DEFAULT now()
);`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return s.InitCounters(ctx)
}

func (s *Worker) InitCounters(ctx context.Context) error {
	const q = `
		INSERT INTO counters (counter_name, counter_value, updated_at)
		VALUES
		  ('import_requests_failed', 0, now()),
		  ('import_requests_successful', 0, now()),
		  ('search_requests_failed', 0, now()),
		  ('search_requests_successful', 0, now())
		ON CONFLICT (counter_name) DO NOTHING;
	`

	_, err := s.db.ExecContext(ctx, q)
	return err
}
