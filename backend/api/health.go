package api

import (
	"log/slog"
	"net/http"
)

func (rs *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	slog.Info("Received request to /health")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
