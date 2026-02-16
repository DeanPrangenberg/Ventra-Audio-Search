package api

import (
	"errors"
	"go_audio_search_api_server/globalTypes"
	"log/slog"
	"net/http"
	"time"
)

type Server struct {
	port           string
	routerTaskChan chan globalTypes.AudioDataElement
	httpServer     *http.Server
}

func NewRestServer(port string, routerTaskChan chan globalTypes.AudioDataElement) *Server {
	rs := &Server{port: port, routerTaskChan: routerTaskChan}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", rs.handleHealth)
	mux.HandleFunc("POST /import", rs.handleImport)

	rs.httpServer = &http.Server{
		Addr:              ":" + rs.port,
		Handler:           logging(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return rs
}

func (rs *Server) Run() error {
	slog.Info("listening", "addr", rs.httpServer.Addr)
	err := rs.httpServer.ListenAndServe() // BLOCKT
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
