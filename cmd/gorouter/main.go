package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/xuando/gorouter/internal/api"
	"github.com/xuando/gorouter/internal/config"
	"github.com/xuando/gorouter/internal/db"
	"github.com/xuando/gorouter/internal/executor"
	"github.com/xuando/gorouter/internal/router"
	"github.com/xuando/gorouter/internal/server"
	"github.com/xuando/gorouter/internal/usage"
)

func main() {
	loadDotEnv()

	cfg := config.Load()

	store, err := db.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	usageDB, err := usage.NewDB(cfg.DataDir, 1000)
	if err != nil {
		log.Fatalf("open usage db: %v", err)
	}
	logger := usage.NewLogger(cfg.DataDir)
	tracker := usage.NewTracker(usageDB, logger, nil)

	executor.Init(cfg)

	h := router.NewHandler(store, tracker)

	srv := server.New(cfg)
	srv.Mount(func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"ok","service":"gorouter"}`))
		})
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
		})

		// Dashboard UI (proxy to Next.js dev server or placeholder)
		dashboardURL := os.Getenv("DASHBOARD_URL")
		var dash http.Handler
		if dashboardURL != "" {
			dash = server.DashboardProxy(dashboardURL)
		} else {
			dash = server.DashboardPlaceholder()
		}
		r.Handle("/dashboard", dash)
		r.Handle("/dashboard/*", dash)

		// Management API
		r.Mount("/api", api.NewRouter(store, cfg, usageDB))

		// OpenAI-compatible routing endpoints
		r.Post("/v1/chat/completions", h.HandleChat)
		r.Post("/v1/messages", h.HandleChat)
		r.Post("/v1/responses", h.HandleChat)
		r.Get("/v1/models", h.HandleModels)
		r.Get("/v1beta/models", h.HandleModels)
	})

	if err := srv.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func loadDotEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for _, line := range splitLines(data) {
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		for i, c := range line {
			if c == '=' {
				key := line[:i]
				val := line[i+1:]
				if os.Getenv(key) == "" {
					os.Setenv(key, val)
				}
				break
			}
		}
	}
}

func splitLines(data []byte) []string {
	var lines []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			line := string(data[start:i])
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, string(data[start:]))
	}
	return lines
}
