package restApi

import (
	"context"
	"encoding/json"
	"errors"
	"go_audio_search_api_server/postgres"
	"log/slog"
	"net/http"
	"time"
)

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info(r.Method, r.URL.Path, time.Since(start).String())
	})
}

func (rs *Server) writeJsonWithCounter(w http.ResponseWriter, status int, counter postgres.Counter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(v)

	ctx, cancel := rs.opCtx()
	err := rs.postgres.AddToCounter(ctx, counter, 1)
	cancel()

	if err != nil {
		slog.Error("Error while updating " + string(counter) + " counter after api call: " + err.Error())
	}
}

func (rs *Server) writeJson(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(v)
}

func ReadJSON(r *http.Request, dst any, maxBytes int64) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBytes)
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return err
	}
	// Ensure no trailing garbage
	if dec.More() {
		return errors.New("invalid json: multiple values")
	}
	return nil
}

func (rs *Server) opCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(rs.StopCtx, opTimeout)
}
