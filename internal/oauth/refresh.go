package oauth

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/xdotech/gorouter/internal/db"
	"github.com/xdotech/gorouter/internal/oauth/providers"
)

// ─── Configuration ──────────────────────────────────────────────────────────

const (
	// tokenExpiryBuffer is how far ahead of actual expiry we refresh.
	tokenExpiryBuffer = 5 * time.Minute

	// maxRefreshRetries is the maximum number of retry attempts.
	maxRefreshRetries = 3
)

// ─── Types ──────────────────────────────────────────────────────────────────

// RefreshResult holds refreshed token data.
type RefreshResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

// ─── Concurrent Refresh Guard ───────────────────────────────────────────────

// inflightRefresh tracks in-progress refresh operations to prevent duplicates.
// Key: connectionID, Value: chan *refreshOutcome
var inflightRefresh sync.Map

type refreshOutcome struct {
	token string
	err   error
}

// ─── Public API ─────────────────────────────────────────────────────────────

// IsTokenExpired returns true if the token expires within the buffer window.
func IsTokenExpired(expiresAt string) bool {
	if expiresAt == "" {
		return false // no expiry set — assume valid (e.g. API key providers)
	}
	t, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return true // can't parse — treat as expired
	}
	return time.Now().UTC().Add(tokenExpiryBuffer).After(t)
}

// EnsureFreshToken checks if a connection's token is expired and refreshes if needed.
// Uses singleflight to prevent concurrent refreshes for the same connection.
// Updates the DB if a refresh occurs. Returns the (possibly refreshed) access token.
func EnsureFreshToken(conn *db.ProviderConnection, store *db.Store) (string, error) {
	if conn.AuthType != "oauth" || conn.RefreshToken == "" {
		return conn.AccessToken, nil
	}

	if !IsTokenExpired(conn.ExpiresAt) {
		return conn.AccessToken, nil
	}

	// Singleflight: if another goroutine is already refreshing this connection, wait.
	ch := make(chan *refreshOutcome, 1)
	if existing, loaded := inflightRefresh.LoadOrStore(conn.ID, ch); loaded {
		slog.Debug("waiting for in-flight refresh", "connID", conn.ID)
		outcome := <-existing.(chan *refreshOutcome)
		// Put the result back so other waiters can also read it.
		existing.(chan *refreshOutcome) <- outcome
		return outcome.token, outcome.err
	}

	// We own this refresh — execute it.
	token, err := doRefresh(conn, store)
	outcome := &refreshOutcome{token: token, err: err}

	// Notify all waiters and clean up.
	ch <- outcome
	inflightRefresh.Delete(conn.ID)

	return token, err
}

// ForceRefresh forces a token refresh (used on 401 reactive refresh).
// Bypasses the expiry check but still uses singleflight.
func ForceRefresh(conn *db.ProviderConnection, store *db.Store) (string, error) {
	if conn.RefreshToken == "" {
		return conn.AccessToken, ErrNoRefreshToken
	}
	return doRefresh(conn, store)
}

// ─── Internal ───────────────────────────────────────────────────────────────

func doRefresh(conn *db.ProviderConnection, store *db.Store) (string, error) {
	slog.Info("refreshing token",
		"provider", conn.Provider,
		"connID", conn.ID,
		"expiresAt", conn.ExpiresAt,
	)

	result, err := refreshWithRetry(conn.Provider, conn.RefreshToken)
	if err != nil {
		classified := ClassifyRefreshError(err)
		slog.Error("token refresh failed",
			"provider", conn.Provider,
			"connID", conn.ID,
			"error", err,
			"permanent", !IsTransientError(err),
		)

		// If token is permanently revoked, deactivate the connection.
		if classified == ErrTokenRevoked {
			slog.Warn("token revoked, deactivating connection", "connID", conn.ID)
			_ = store.UpdateProviderConnection(conn.ID, map[string]interface{}{
				"testStatus":  "unavailable",
				"lastError":   "token revoked or expired — re-authenticate required",
				"lastErrorAt": time.Now().UTC().Format(time.RFC3339),
			})
		}
		return conn.AccessToken, err
	}

	// Persist refreshed tokens.
	var expiresAt string
	if result.ExpiresIn > 0 {
		expiresAt = time.Now().UTC().Add(time.Duration(result.ExpiresIn) * time.Second).Format(time.RFC3339)
	}

	updates := map[string]interface{}{
		"accessToken":  result.AccessToken,
		"refreshToken": result.RefreshToken,
		"expiresAt":    expiresAt,
	}

	// GitHub: also refresh Copilot token.
	if conn.Provider == "github" && result.AccessToken != "" {
		copilotToken, copilotExpiry, err := providers.FetchCopilotToken(result.AccessToken)
		if err != nil {
			slog.Warn("copilot token re-exchange failed", "error", err)
		} else {
			updates["providerSpecificData"] = map[string]interface{}{
				"copilotToken": copilotToken,
				"expiresAt":    copilotExpiry,
			}
		}
	}

	if err := store.UpdateProviderConnection(conn.ID, updates); err != nil {
		slog.Error("failed to persist refreshed token", "error", err)
	}

	slog.Info("token refreshed successfully",
		"provider", conn.Provider,
		"expiresIn", result.ExpiresIn,
	)
	return result.AccessToken, nil
}

// refreshWithRetry attempts token refresh with linear backoff (1s, 2s, 3s).
// Permanent errors (invalid_grant, 400, 401) are NOT retried.
func refreshWithRetry(provider, refreshToken string) (*RefreshResult, error) {
	var lastErr error

	for attempt := 0; attempt < maxRefreshRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(attempt) * time.Second
			slog.Debug("refresh retry",
				"provider", provider,
				"attempt", attempt+1,
				"maxRetries", maxRefreshRetries,
				"delay", delay,
			)
			time.Sleep(delay)
		}

		result, err := refreshOnce(provider, refreshToken)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Don't retry permanent errors.
		if !IsTransientError(err) {
			slog.Debug("permanent error, skipping retries", "error", err)
			return nil, err
		}

		slog.Warn("refresh attempt failed",
			"provider", provider,
			"attempt", attempt+1,
			"error", err,
		)
	}

	return nil, fmt.Errorf("all %d refresh attempts failed: %w", maxRefreshRetries, lastErr)
}

// refreshOnce performs a single token refresh for a given provider.
func refreshOnce(provider, refreshToken string) (*RefreshResult, error) {
	if refreshToken == "" {
		return nil, ErrNoRefreshToken
	}

	switch provider {
	case "claude-code":
		tokens, err := providers.RefreshClaudeToken(refreshToken)
		if err != nil {
			return nil, err
		}
		return &RefreshResult{
			AccessToken:  tokens.AccessToken,
			RefreshToken: orDefault(tokens.RefreshToken, refreshToken),
			ExpiresIn:    tokens.ExpiresIn,
		}, nil

	case "gemini-cli":
		tokens, err := providers.RefreshGeminiToken(refreshToken)
		if err != nil {
			return nil, err
		}
		return &RefreshResult{
			AccessToken:  tokens.AccessToken,
			RefreshToken: orDefault(tokens.RefreshToken, refreshToken),
			ExpiresIn:    tokens.ExpiresIn,
		}, nil

	case "codex":
		tokens, err := providers.RefreshCodexToken(refreshToken)
		if err != nil {
			return nil, err
		}
		return &RefreshResult{
			AccessToken:  tokens.AccessToken,
			RefreshToken: orDefault(tokens.RefreshToken, refreshToken),
			ExpiresIn:    tokens.ExpiresIn,
		}, nil

	case "antigravity":
		tokens, err := providers.RefreshAntigravityToken(refreshToken)
		if err != nil {
			return nil, err
		}
		return &RefreshResult{
			AccessToken:  tokens.AccessToken,
			RefreshToken: orDefault(tokens.RefreshToken, refreshToken),
			ExpiresIn:    tokens.ExpiresIn,
		}, nil

	case "qwen":
		at, rt, err := providers.RefreshQwenToken(refreshToken)
		if err != nil {
			return nil, err
		}
		return &RefreshResult{
			AccessToken:  at,
			RefreshToken: orDefault(rt, refreshToken),
		}, nil

	case "iflow":
		at, rt, err := providers.RefreshIFlowToken(refreshToken)
		if err != nil {
			return nil, err
		}
		return &RefreshResult{
			AccessToken:  at,
			RefreshToken: orDefault(rt, refreshToken),
		}, nil

	case "github":
		// GitHub doesn't have refresh tokens — re-exchange for Copilot token.
		// The GH access token doesn't expire, but the Copilot token does.
		// Return the same access token; Copilot re-exchange happens in doRefresh.
		return &RefreshResult{
			AccessToken:  refreshToken, // GH access token is stored as refreshToken
			RefreshToken: refreshToken,
		}, nil

	default:
		return nil, ErrRefreshUnsupported
	}
}

func orDefault(val, fallback string) string {
	if val != "" {
		return val
	}
	return fallback
}
