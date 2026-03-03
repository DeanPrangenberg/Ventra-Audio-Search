package main

import (
	"fmt"
	"go_audio_search_api_server/AudioDataRouter"
	"go_audio_search_api_server/api"
	"log/slog"
	"os"
	"strings"
)

func loadEnv(key string) (string, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return "", fmt.Errorf("env: %s doesn't exist", key)
	}
	return v, nil
}

func initLogger() {
	log, err := loadEnv("LOG_LEVEL")
	if err != nil {
		log = "info"
	}

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

	router := AudioDataRouter.NewRoutWorker("/app/sqlite/database.db", 12)

	srv := api.NewRestServer("8880", router)
	if err := srv.Run(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
