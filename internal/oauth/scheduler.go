package oauth

import (
	"log/slog"
	"time"

	"github.com/xuando/gorouter/internal/db"
)

// ─── Background Token Refresh Scheduler ─────────────────────────────────────

const (
	// refreshInterval is how often the scheduler scans for expiring tokens.
	refreshInterval = 4 * time.Minute
)

// StartScheduler launches a background goroutine that proactively refreshes
// OAuth tokens before they expire. This prevents request-time latency from
// token refresh and ensures tokens are always fresh.
//
// The goroutine runs every 4 minutes, scanning all OAuth connections for tokens
// expiring within the 5-minute buffer window. It stops when the done channel
// is closed (e.g. on server shutdown).
func StartScheduler(store *db.Store, done <-chan struct{}) {
	go func() {
		slog.Info("token refresh scheduler started", "interval", refreshInterval)

		// Run once immediately on startup.
		refreshAllExpiring(store)

		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				slog.Info("token refresh scheduler stopped")
				return
			case <-ticker.C:
				refreshAllExpiring(store)
			}
		}
	}()
}

// refreshAllExpiring scans all OAuth connections and refreshes expiring tokens.
func refreshAllExpiring(store *db.Store) {
	connections, err := store.GetProviderConnections(db.ConnectionFilter{})
	if err != nil {
		slog.Error("scheduler: failed to list connections", "error", err)
		return
	}

	refreshed := 0
	for _, conn := range connections {
		if conn.AuthType != "oauth" || conn.RefreshToken == "" {
			continue
		}

		if !IsTokenExpired(conn.ExpiresAt) {
			continue
		}

		slog.Debug("scheduler: refreshing expiring token",
			"provider", conn.Provider,
			"connID", conn.ID,
			"expiresAt", conn.ExpiresAt,
		)

		connCopy := conn // avoid loop variable capture
		if _, err := EnsureFreshToken(&connCopy, store); err != nil {
			slog.Warn("scheduler: refresh failed",
				"provider", conn.Provider,
				"connID", conn.ID,
				"error", err,
			)
			continue
		}
		refreshed++
	}

	if refreshed > 0 {
		slog.Info("scheduler: proactive refresh complete", "refreshed", refreshed)
	}
}
