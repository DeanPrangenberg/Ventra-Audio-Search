package sqlite

import (
	"context"
	"database/sql"
	"log/slog"
)

type SQLiteStore struct {
	db *sql.DB
}

func newSQLiteStore(db *sql.DB) *SQLiteStore {
	sqlite := SQLiteStore{db: db}

	ctx := context.Background()

	err := sqlite.CreateTables(ctx)
	if err != nil {
		slog.Error("Could not open DB:")
		panic(err)
	}

	return &sqlite
}

func Open(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	// sensible defaults
	if _, err := db.Exec(`PRAGMA foreign_keys=ON;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	return newSQLiteStore(db), nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

/*
ctx := context.Background()

store, err := audiosql.Open("./audio.sqlite")
if err != nil { panic(err) }
defer store.Close()

if err := store.CreateTables(ctx); err != nil { panic(err) }

a := &audiosql.AudioDataElement{
	AudiofileHash:   "hash123",
	Title:           "Meeting 12.02",
	RecordingDate:   "2026-02-12",
	FileUrl:         "https://example",
	DownloadPath:    "/data/hash123.mp3",
	DurationInSec:   3600,
	UserSummaryText: "Kurze Notizen vom User",
	AIKeywords:      []string{"kubernetes", "deadline", "incident"},
	AISummary:       "AI Summary text ...",
}
if err := store.UpsertBase(ctx, a); err != nil { panic(err) }

segs := []audiosql.SegmentElement{
	{SegmentHash: "seg1", AudiofileHash: a.AudiofileHash, StartInSec: 0, EndInSec: 60, Transcript: "blabla"},
	{SegmentHash: "seg2", AudiofileHash: a.AudiofileHash, StartInSec: 60, EndInSec: 120, Transcript: "blabla2"},
}
if err := store.InsertSegmentsUpsert(ctx, a.AudiofileHash, segs); err != nil { panic(err) }
*/
