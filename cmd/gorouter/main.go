package main

import (
	"context"
	"log"
	"os"

	"github.com/xdotech/gorouter/internal/config"
	"github.com/xdotech/gorouter/internal/db"
	"github.com/xdotech/gorouter/internal/domain"
	"github.com/xdotech/gorouter/internal/executor"
	"github.com/xdotech/gorouter/internal/gateway"
	"github.com/xdotech/gorouter/internal/lifecycle"
	"github.com/xdotech/gorouter/internal/logging"
	"github.com/xdotech/gorouter/internal/storage/jsonfile"
	"github.com/xdotech/gorouter/internal/usage"
)

func main() {
	loadDotEnv()

	cfg := config.Load()

	logging.Setup("info")
	logger := logging.FromContext(context.Background())
	logger.Info("initializing gorouter")

	// ─── Legacy Store (used by router.Handler and oauth) ─────────────
	dbStore, err := db.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	// ─── Domain Stores (used by gateway handlers) ─────────────────────
	_, stores, err := jsonfile.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("open domain stores: %v", err)
	}

	// ─── Usage ────────────────────────────────────────────────────────
	usageDB, err := usage.NewDB(cfg.DataDir, 1000)
	if err != nil {
		log.Fatalf("open usage db: %v", err)
	}
	usageLogger := usage.NewLogger(cfg.DataDir)
	tracker := usage.NewTracker(usageDB, usageLogger, nil)

	// ─── Executors ────────────────────────────────────────────────────
	executor.Init(cfg)

	// ─── Gateway Server ───────────────────────────────────────────────
	srv := gateway.NewServer(cfg, stores, dbStore, tracker, usageDB)

	// ─── Service Group ────────────────────────────────────────────────
	sg := lifecycle.NewServiceGroup()
	sg.Add(srv)

	logger.Info("starting gorouter", "addr", ":"+cfg.Port)
	if err := sg.Start(context.Background()); err != nil {
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

// Ensure domain.Stores is used (prevents "imported and not used" error).
var _ = (*domain.Stores)(nil)
