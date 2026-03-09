package api

import (
	"context"
	"errors"
	"go_audio_search_api_server/globalUtils"
	"go_audio_search_api_server/postgres"
	"go_audio_search_api_server/searcher"
	"log/slog"
	"net/http"
	"time"
)

const opTimeout = 1 * time.Minute

type Server struct {
	port             string
	PoolRefillSignal *globalUtils.NoneStackingEvent
	StopCtx          context.Context
	searcher         *searcher.Worker
	httpServer       *http.Server
	postgres         *postgres.Worker
}

func NewRestServer(ctx context.Context, port string, postgres *postgres.Worker, searcher *searcher.Worker, poolRefillSignal *globalUtils.NoneStackingEvent) *Server {
	rs := &Server{
		port:             port,
		PoolRefillSignal: poolRefillSignal,
		StopCtx:          ctx,
		searcher:         searcher,
		postgres:         postgres,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", rs.handleHealth)
	mux.HandleFunc("POST /import", rs.handleImport)
	mux.HandleFunc("GET /search", rs.handleSearch)

	rs.httpServer = &http.Server{
		Addr:              ":" + rs.port,
		Handler:           logging(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return rs
}

func (rs *Server) Run() error {
	slog.Info("listening", "addr", rs.httpServer.Addr)
	err := rs.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (rs *Server) Shutdown(ctx context.Context) error {
	slog.Info("Shutting down REST server...")

	err := rs.httpServer.Shutdown(ctx)
	if err != nil {
		slog.Error("Error shutting down REST server", "err", err)
		return err
	}

	return nil
}
