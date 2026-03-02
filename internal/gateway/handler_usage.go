package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/xdotech/gorouter/internal/usage"
)

func (s *Server) handleUsageProviders() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats := s.usageDB.Aggregate()
		renderJSON(w, http.StatusOK, stats)
	}
}

func (s *Server) handleUsageRequestLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if page < 1 {
			page = 1
		}
		if limit < 1 {
			limit = 20
		}
		entries, total := s.usageDB.GetPage(page, limit)
		renderJSON(w, http.StatusOK, map[string]interface{}{
			"entries": entries,
			"total":   total,
			"page":    page,
			"limit":   limit,
		})
	}
}

func (s *Server) handleUsageStream() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			renderError(w, http.StatusInternalServerError, "streaming not supported")
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		ch := usage.Global.Subscribe()
		defer usage.Global.Unsubscribe(ch)

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
}
