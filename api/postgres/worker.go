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

type Worker struct {
	db *sql.DB
}

func newPostgresWrapper(db *sql.DB) *Worker {
	store := Worker{db: db}

	ctx := context.Background()
	if err := store.CreateTables(ctx); err != nil {
		slog.Error("Could not initialize Postgres schema")
		panic(err)
	}

	return &store
}

func Open() (*Worker, error) {
	postgresUser := globalUtils.LoadEnvStr("POSTGRES_USER")
	postgresPassword := globalUtils.LoadEnvStr("POSTGRES_PASSWORD")
	postgresHost := globalUtils.LoadEnvStr("POSTGRES_URL")
	postgresDB := globalUtils.LoadEnvStr("POSTGRES_DB")

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
		time.Sleep(3 * time.Second)
		db, err = sql.Open("pgx", postgresConnection)
		if err != nil {
			slog.Warn("sql.Open failed", "err", err)
			time.Sleep(3 * time.Second)
			continue
		}

		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)
		db.SetConnMaxIdleTime(10 * time.Minute)
		db.SetConnMaxLifetime(60 * time.Minute)

		err = db.Ping()
		if err == nil {
			slog.Info("postgres connection established")
			return newPostgresWrapper(db), nil
		}

		slog.Warn("db.Ping failed", "err", err, "dsn", postgresConnection)
		_ = db.Close()
	}

	panic("Could not connect to postgres. err: " + err.Error())

	return nil, fmt.Errorf("could not connect to postgres after retries: %w", err)
}

func (s *Worker) Close() error {
	return s.db.Close()
}
