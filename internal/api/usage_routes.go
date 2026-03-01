package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/xdotech/gorouter/internal/usage"
)

// UsageHandler serves usage stats, logs, and SSE stream.
type UsageHandler struct {
	usageDB     *usage.DB
	broadcaster *usage.Broadcaster
}

func NewUsageHandler(usageDB *usage.DB, broadcaster *usage.Broadcaster) *UsageHandler {
	return &UsageHandler{usageDB: usageDB, broadcaster: broadcaster}
}

// Providers GET /api/usage/providers
func (h *UsageHandler) Providers(w http.ResponseWriter, r *http.Request) {
	stats := h.usageDB.Aggregate()
	JSON(w, http.StatusOK, stats)
}

// RequestLogs GET /api/usage/request-logs?page=1&limit=20
func (h *UsageHandler) RequestLogs(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	entries, total := h.usageDB.GetPage(page, limit)
	JSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

// Stream GET /api/usage/stream (SSE)
func (h *UsageHandler) Stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		Error(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := h.broadcaster.Subscribe()
	defer h.broadcaster.Unsubscribe(ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case entry, open := <-ch:
			if !open {
				return
			}
			data, err := json.Marshal(entry)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
