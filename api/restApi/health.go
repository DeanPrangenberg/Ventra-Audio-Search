package restApi

import (
	"log/slog"
	"net/http"
)

func (rs *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	slog.Info("Received request to /health")
	rs.writeJson(w, http.StatusOK, map[string]any{"ok": true})
}
