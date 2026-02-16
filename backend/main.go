package main

import (
    "fmt"
    "go_audio_search_api_server/AudioDataRouter"
    "go_audio_search_api_server/api"
    "log/slog"
    "os"
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
        log = "INFO"
    }

    var logLevel = log
    var level slog.Level

    switch logLevel {
    case "DEBUG":
        level = slog.LevelDebug
    case "INFO":
        level = slog.LevelInfo
    case "WARN":
        level = slog.LevelWarn
    case "ERROR":
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

    srv := api.NewRestServer("8880", router.TaskChan)
    if err := srv.Run(); err != nil {
        slog.Error("server stopped", "err", err)
        os.Exit(1)
    }
}
