package api

import (
	"errors"
	"go_audio_search_api_server/AudioDataRouter"
	"go_audio_search_api_server/globalTypes"
	"log/slog"
	"net/http"
	"time"
)

type Server struct {
	port           string
	importTaskChan chan globalTypes.AudioDataElement
	searchTaskChan chan globalTypes.SearchRequest
	httpServer     *http.Server
}

func NewRestServer(port string, router *AudioDataRouter.RoutWorker) *Server {
	rs := &Server{
		port:           port,
		importTaskChan: router.ImportTaskChan,
		searchTaskChan: router.SearchTaskChan,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", rs.handleHealth)
	mux.HandleFunc("POST /import", rs.handleImport)
	mux.HandleFunc("POST /search", rs.handleSearch)

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
