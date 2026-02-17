package api

import (
	"fmt"
	"go_audio_search_api_server/globalTypes"
	"log/slog"
	"net/http"
	"strings"
)

func (rs *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	slog.Info("Received request to /import")

	// 1) Content-Type hart prüfen -> 415
	ct := r.Header.Get("Content-Type")
	if ct == "" || !strings.HasPrefix(ct, "application/json") && !strings.HasPrefix(ct, "application/json; charset=utf-8") {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{
			"ok":    false,
			"code":  "IMPORT_UNSUPPORTED_CONTENT_TYPE",
			"error": "Content-Type must be application/json",
			"got":   ct,
		})
		return
	}

	// 2) JSON lesen + limit -> 400/413
	var req []globalTypes.AudioDataElement

	// 1000 MiB
	const maxBody = 1000 * (1 << 20)

	if err := ReadJSON(r, &req, maxBody); err != nil {
		// Wenn du ReadJSON nicht kontrollierst: wir mappen zumindest häufige Fälle.
		// - zu groß -> 413
		// - sonst -> 400
		msg := err.Error()

		// grobes Heuristik-Mapping (besser wäre: ReadJSON gibt sentinel error zurück)
		if strings.Contains(strings.ToLower(msg), "too large") ||
			strings.Contains(strings.ToLower(msg), "request body too large") ||
			strings.Contains(strings.ToLower(msg), "http: request body too large") {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
				"ok":    false,
				"code":  "IMPORT_PAYLOAD_TOO_LARGE",
				"error": "Request body too large",
				"limit": maxBody,
			})
			return
		}

		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"code":  "IMPORT_BAD_JSON",
			"error": msg,
		})
		return
	}

	// 3) Validieren -> 422 / 207 / 200
	validItems := make([]globalTypes.AudioDataElement, 0, len(req))
	invalidItemsIdx := make([]int, 0, len(req))
	invalidItemsErr := make([]string, 0, len(req))

	for idx, item := range req {
		if err := item.ValidateApiInput(); err != nil {
			slog.Debug("Received invalid item via api: " + item.ToString())
			invalidItemsIdx = append(invalidItemsIdx, idx)
			invalidItemsErr = append(invalidItemsErr, err.Error())
			continue
		}
		validItems = append(validItems, item)
	}

	// alle invalid -> 422
	if len(validItems) == 0 {
		slog.Info("Received an import with no valid items to import")
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"ok":    false,
			"code":  "IMPORT_VALIDATION_FAILED",
			"error": "No valid items in request",
			"invalid": map[string]any{
				"count":   len(invalidItemsIdx),
				"indexes": invalidItemsIdx,
				"errors":  invalidItemsErr,
			},
		})
		return
	}

	// mixed -> 207
	if len(invalidItemsIdx) > 0 {
		errMsg := fmt.Sprintf(
			"Received an import request with %d valid and %d invalid items",
			len(validItems),
			len(invalidItemsIdx),
		)
		slog.Info(errMsg)

		writeJSON(w, http.StatusMultiStatus, map[string]any{
			"ok":    false,
			"code":  "IMPORT_PARTIAL",
			"error": "Some items were rejected",
			"valid": map[string]any{
				"count": len(validItems),
			},
			"invalid": map[string]any{
				"count":   len(invalidItemsIdx),
				"indexes": invalidItemsIdx,
				"errors":  invalidItemsErr,
			},
		})
		return
	}

	for _, item := range validItems {
		slog.Info("Queueing item for processing: " + item.Title)
		rs.importTaskChan <- item
	}

	// alles valid -> 200
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"imported": map[string]any{
			"count": len(validItems),
		},
	})
}
