package main

import (
	"go_audio_search_api_server/FlowManager"
	"go_audio_search_api_server/api"
	"go_audio_search_api_server/globalUtils"
	"log/slog"
	"os"
	"strings"
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

	router := FlowManager.NewRoutWorker(12)

	srv := api.NewRestServer("8880", router)
	if err := srv.Run(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
