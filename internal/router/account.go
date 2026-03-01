package router

import (
	"fmt"
	"sync"
	"time"

	"github.com/xuando/gorouter/internal/db"
	"github.com/xuando/gorouter/internal/oauth"
)

var selectionMu sync.Mutex

// modelLocks stores expiry times for per-model rate limits.
// key: "connId:model" → expiry time.Time
var modelLocks sync.Map

// multiBucketProviders are providers where rate limits are per model bucket.
var multiBucketProviders = map[string]bool{"antigravity": true}

// SelectedAccount holds the resolved credentials for a selected connection.
type SelectedAccount struct {
	ConnectionID         string
	APIKey               string
	AccessToken          string
	RefreshToken         string
	ExpiresAt            string
	ProjectID            string
	CopilotToken         string
	ProviderSpecificData map[string]interface{}
	TestStatus           string
	BackoffLevel         int
	RateLimitedUntil     string
}

// SelectAccount picks the best available account for a provider.
// excludeID skips a specific connection (used after fallback).
// model is used for per-model lock checks on multi-bucket providers.
func SelectAccount(provider, excludeID, model string, store *db.Store) (*SelectedAccount, error) {
	selectionMu.Lock()
	defer selectionMu.Unlock()

	active := true
	conns, err := store.GetProviderConnections(db.ConnectionFilter{
		Provider: provider,
		IsActive: &active,
	})
	if err != nil {
		return nil, fmt.Errorf("select account: %w", err)
	}

	settings, err := store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("select account settings: %w", err)
	}

	// Filter out unavailable and excluded connections.
	var available []db.ProviderConnection
	for _, c := range conns {
		if c.ID == excludeID {
			continue
		}
		if !IsAccountAvailable(c.RateLimitedUntil) {
			continue
		}
		if multiBucketProviders[provider] && model != "" {
			if isModelLocked(c.ID, model) {
				continue
			}
		}
		available = append(available, c)
	}

	if len(available) == 0 {
		return nil, nil
	}

	var chosen db.ProviderConnection
	switch settings.FallbackStrategy {
	case "round-robin":
		chosen = selectRoundRobin(available, settings.StickyRoundRobinLimit)
	default: // "fill-first"
		chosen = selectFillFirst(available)
	}

	// Update last used and consecutive count.
	_ = store.UpdateProviderConnection(chosen.ID, map[string]interface{}{
		"lastUsedAt":          db.Now(),
		"consecutiveUseCount": chosen.ConsecutiveUseCount + 1,
	})

	copilotToken := ""
	if chosen.ProviderSpecificData != nil {
		if v, ok := chosen.ProviderSpecificData["copilotToken"].(string); ok {
			copilotToken = v
		}
	}

	// Auto-refresh OAuth tokens if expired/expiring.
	accessToken := chosen.AccessToken
	if chosen.AuthType == "oauth" && chosen.RefreshToken != "" {
		freshToken, err := oauth.EnsureFreshToken(&chosen, store)
		if err == nil {
			accessToken = freshToken
		}
	}

	return &SelectedAccount{
		ConnectionID:         chosen.ID,
		APIKey:               chosen.APIKey,
		AccessToken:          accessToken,
		RefreshToken:         chosen.RefreshToken,
		ExpiresAt:            chosen.ExpiresAt,
		ProjectID:            chosen.ProjectID,
		CopilotToken:         copilotToken,
		ProviderSpecificData: chosen.ProviderSpecificData,
		TestStatus:           chosen.TestStatus,
		BackoffLevel:         chosen.BackoffLevel,
		RateLimitedUntil:     chosen.RateLimitedUntil,
	}, nil
}

// selectFillFirst picks the highest-priority connection (lowest priority number),
// filling it until exhausted. Ties broken by consecutive use count.
func selectFillFirst(conns []db.ProviderConnection) db.ProviderConnection {
	best := conns[0]
	for _, c := range conns[1:] {
		if c.Priority < best.Priority {
			best = c
		} else if c.Priority == best.Priority && c.ConsecutiveUseCount < best.ConsecutiveUseCount {
			best = c
		}
	}
	return best
}

// selectRoundRobin picks based on sticky round-robin within stickyLimit.
func selectRoundRobin(conns []db.ProviderConnection, stickyLimit int) db.ProviderConnection {
	// Use the one with fewest consecutive uses (or first if all equal).
	best := conns[0]
	for _, c := range conns[1:] {
		if c.ConsecutiveUseCount < best.ConsecutiveUseCount {
			best = c
		}
	}
	// If sticky limit reached, move to next.
	if stickyLimit > 0 && best.ConsecutiveUseCount >= stickyLimit {
		for _, c := range conns {
			if c.ID != best.ID {
				return c
			}
		}
	}
	return best
}

// IsAccountAvailable returns true if rateLimitedUntil is empty or in the past.
func IsAccountAvailable(rateLimitedUntil string) bool {
	if rateLimitedUntil == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, rateLimitedUntil)
	if err != nil {
		return true
	}
	return time.Now().UTC().After(t)
}

// MarkAccountUnavailable applies backoff and marks the connection rate-limited.
// Returns whether fallback should occur.
func MarkAccountUnavailable(connID string, statusCode int, errorText, provider, model string, store *db.Store) (bool, error) {
	conn, err := store.GetProviderConnection(connID)
	if err != nil {
		return false, err
	}

	decision := CheckFallbackError(statusCode, errorText, conn.BackoffLevel)
	if !decision.ShouldFallback {
		return false, nil
	}

	until := time.Now().UTC().Add(time.Duration(decision.CooldownMs) * time.Millisecond).Format(time.RFC3339)

	updates := map[string]interface{}{
		"rateLimitedUntil": until,
		"backoffLevel":     decision.NewBackoffLevel,
		"testStatus":       "unavailable",
		"lastError":        errorText,
		"errorCode":        statusCode,
		"lastErrorAt":      db.Now(),
	}
	if err := store.UpdateProviderConnection(connID, updates); err != nil {
		return true, err
	}

	// Lock per-model for multi-bucket providers.
	if multiBucketProviders[provider] && model != "" {
		lockModel(connID, model, time.Duration(decision.CooldownMs)*time.Millisecond)
	}

	return true, nil
}

// ClearAccountError resets error state after a successful request.
func ClearAccountError(connID string, store *db.Store) {
	_ = store.UpdateProviderConnection(connID, map[string]interface{}{
		"rateLimitedUntil":    nil,
		"backoffLevel":        0,
		"testStatus":          "active",
		"lastError":           nil,
		"errorCode":           0,
		"consecutiveUseCount": 0,
	})
}

// isModelLocked checks if a model is currently rate-limited for a connection.
func isModelLocked(connID, model string) bool {
	key := connID + ":" + model
	v, ok := modelLocks.Load(key)
	if !ok {
		return false
	}
	expiry, ok := v.(time.Time)
	if !ok {
		return false
	}
	if time.Now().UTC().After(expiry) {
		modelLocks.Delete(key)
		return false
	}
	return true
}

// lockModel sets an in-memory rate limit for a specific model on a connection.
func lockModel(connID, model string, duration time.Duration) {
	key := connID + ":" + model
	modelLocks.Store(key, time.Now().UTC().Add(duration))
}
