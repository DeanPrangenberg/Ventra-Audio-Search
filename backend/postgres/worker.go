package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"go_audio_search_api_server/globalUtils"
	"log/slog"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgressWrapper struct {
	db *sql.DB
}

func newPostgresWrapper(db *sql.DB) *PostgressWrapper {
	store := PostgressWrapper{db: db}

	ctx := context.Background()
	if err := store.CreateTables(ctx); err != nil {
		slog.Error("Could not initialize Postgres schema")
		panic(err)
	}

	return &store
}

func Open() (*PostgressWrapper, error) {
	postgresUser := globalUtils.LoadEnv("POSTGRES_USER")
	postgresPassword := globalUtils.LoadEnv("POSTGRES_PASSWORD")
	postgresHost := globalUtils.LoadEnv("POSTGRES_URL")
	postgresDB := globalUtils.LoadEnv("POSTGRES_DB")

	if postgresUser == "" {
		return nil, errors.New("POSTGRES_USER is empty")
	}
	if postgresPassword == "" {
		return nil, errors.New("POSTGRES_PASSWORD is empty")
	}
	if postgresHost == "" {
		return nil, errors.New("POSTGRES_URL is empty")
	}
	if postgresDB == "" {
		return nil, errors.New("POSTGRES_DB is empty")
	}

	postgresConnection := fmt.Sprintf(
		"postgres://%s:%s@%s/%s?sslmode=disable",
		postgresUser,
		postgresPassword,
		postgresHost,
		postgresDB,
	)

	var db *sql.DB
	var err error

	for range 10 {
		db, err = sql.Open("pgx", postgresConnection)
		if err != nil {
			slog.Error("sql.Open failed", "err", err)
			time.Sleep(3 * time.Second)
			continue
		}

		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)
		db.SetConnMaxIdleTime(10 * time.Minute)
		db.SetConnMaxLifetime(60 * time.Minute)

		err = db.Ping()
		if err == nil {
			return newPostgresWrapper(db), nil
		}

		slog.Error("db.Ping failed", "err", err, "dsn", postgresConnection)
		_ = db.Close()
		time.Sleep(3 * time.Second)
	}

	return nil, fmt.Errorf("could not connect to postgres after retries: %w", err)
}

func (s *PostgressWrapper) Close() error { return s.db.Close() }
