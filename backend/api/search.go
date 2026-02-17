package api

import (
	"go_audio_search_api_server/globalTypes"
	"log/slog"
	"net/http"
	"strings"
)

func (rs *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	slog.Info("Received request to /Search")

	// 1) Content-Type hart prüfen -> 415
	ct := r.Header.Get("Content-Type")
	if ct == "" || !strings.HasPrefix(ct, "application/json") && !strings.HasPrefix(ct, "application/json; charset=utf-8") {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{
			"ok":    false,
			"code":  "SEARCH_UNSUPPORTED_CONTENT_TYPE",
			"error": "Content-Type must be \"application/json\" or \"application/json; charset=utf-8\"",
			"got":   ct,
		})
		return
	}

	// 2) JSON lesen + limit -> 400/413
	var searchRequest globalTypes.SearchRequest

	// 10 MiB
	const maxBody = 10 * (1 << 20)

	if err := ReadJSON(r, &searchRequest, maxBody); err != nil {
		msg := err.Error()

		if strings.Contains(strings.ToLower(msg), "too large") ||
			strings.Contains(strings.ToLower(msg), "request body too large") ||
			strings.Contains(strings.ToLower(msg), "http: request body too large") {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
				"ok":    false,
				"code":  "SEARCH_PAYLOAD_TOO_LARGE",
				"error": "Request body too large",
				"limit": maxBody,
			})
			return
		}

		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"code":  "SEARCH_BAD_JSON",
			"error": msg,
		})
		return
	}

	err := searchRequest.ValidateApiInput()
	if err != nil {
		slog.Info("Received an search with invalid parameters")
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"ok":    false,
			"code":  "SEARCH_VALIDATION_FAILED",
			"error": "Search request has a invalid parameter: " + err.Error(),
		})
	}

	searchRequest.BackendResponseChan = make(chan globalTypes.SearchResponse, 1)

	slog.Info("Queueing Search item for processing Query: " + searchRequest.SemanticSearchQuery)
	rs.searchTaskChan <- searchRequest

	res := <-searchRequest.BackendResponseChan

	close(searchRequest.BackendResponseChan)

	var status int
	if res.Ok {
		status = http.StatusOK
	} else {
		status = http.StatusConflict
	}

	// Return response
	writeJSON(w, status, res)
}
