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
