package postgres

import (
	"context"
	"database/sql"
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
	postgresUrl := globalUtils.LoadEnv("POSTGRES_URL")
	postgresDB := globalUtils.LoadEnv("POSTGRES_DB")

	postgresConnection := fmt.Sprintf(
		"postgres://%s:%s@%s/%s?sslmode=disable",
		postgresUser, postgresPassword, postgresUrl, postgresDB,
	)
	var db *sql.DB
	var err error
	for _ = range 10 {
		db, err = sql.Open("pgx", postgresConnection)
		if err == nil {
			break
		}

		time.Sleep(10 * time.Second)
	}

	if err != nil || db == nil {
		panic("Couldn't connect to DB: " + postgresConnection)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(10 * time.Minute)
	db.SetConnMaxLifetime(60 * time.Minute)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return newPostgresWrapper(db), nil
}

func (s *PostgressWrapper) Close() error { return s.db.Close() }
