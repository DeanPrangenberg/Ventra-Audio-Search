package restApi

import (
	"go_audio_search_api_server/globalTypes"
	"go_audio_search_api_server/postgres"
	"log/slog"
	"net/http"
	"strings"
)

func (rs *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	slog.Info("Received request to /search")

	// 1) Content-Type hart prüfen -> 415
	ct := r.Header.Get("Content-Type")
	if ct == "" || !strings.HasPrefix(ct, "application/json") && !strings.HasPrefix(ct, "application/json; charset=utf-8") {
		rs.writeJsonWithCounter(w, http.StatusUnsupportedMediaType, postgres.SearchRequestsFailed, map[string]any{
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
			rs.writeJsonWithCounter(w, http.StatusRequestEntityTooLarge, postgres.SearchRequestsFailed, map[string]any{
				"ok":    false,
				"code":  "SEARCH_PAYLOAD_TOO_LARGE",
				"error": "Request body too large",
				"limit": maxBody,
			})
			return
		}

		rs.writeJsonWithCounter(w, http.StatusBadRequest, postgres.SearchRequestsFailed, map[string]any{
			"ok":    false,
			"code":  "SEARCH_BAD_JSON",
			"error": msg,
		})
		return
	}

	err := searchRequest.ValidateApiInput()
	if err != nil {
		slog.Info("Received an search with invalid parameters")
		rs.writeJsonWithCounter(w, http.StatusUnprocessableEntity, postgres.SearchRequestsFailed, map[string]any{
			"ok":    false,
			"code":  "SEARCH_VALIDATION_FAILED",
			"error": "Search request has a invalid parameter: " + err.Error(),
		})

		return
	}

	slog.Info("Start Search for Query: " + searchRequest.SemanticSearchQuery)

	res := rs.searcher.Search(searchRequest)

	var status int
	if res.Ok {
		status = http.StatusOK
	} else {
		status = http.StatusConflict
	}

	// Return response
	rs.writeJsonWithCounter(w, status, postgres.SearchRequestsSuccessful, res)
}
