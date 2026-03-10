package main

import (
	"context"
	"errors"
	"go_audio_search_api_server/ai"
	"go_audio_search_api_server/api"
	"go_audio_search_api_server/globalUtils"
	"go_audio_search_api_server/importer"
	"go_audio_search_api_server/postgres"
	"go_audio_search_api_server/qdrant"
	"go_audio_search_api_server/searcher"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

func initLogger() {
	log := globalUtils.LoadEnvStr("LOG_LEVEL")

	var logLevel = strings.ToLower(log)
	var level slog.Level

	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	slog.SetDefault(slog.New(handler))
}

func main() {
	initLogger()
	slog.Info("Starting Background workers...")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup

	db, err := postgres.Open()
	if err != nil {
		slog.Error("failed to open db", "err", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("failed to close db gracefully", "err", err)
		}
	}()

	qdrantWorker, err := qdrant.New("AudioSegments")
	if err != nil {
		slog.Error("failed to connect to qdrant", "err", err)
		os.Exit(1)
	}

	poolRefillSignal := globalUtils.NewSignal()
	embedder := ai.NewEmbeddingsWorker()

	importer.NewWorker(ctx, &wg, qdrantWorker, db, embedder, poolRefillSignal)
	searchWorker := searcher.NewWorker(ctx, &wg, qdrantWorker, db, embedder)

	srv := api.NewRestServer(ctx, "8880", db, searchWorker, poolRefillSignal)

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := srv.Run()

		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server stopped", "err", err)
		} else {
			slog.Info("server stopped")
		}

		stop()
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server shutdown failed", "err", err)
		}
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		slog.Error("shutdown timeout")
	}
}
