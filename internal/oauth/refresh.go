package oauth

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/xuando/gorouter/internal/db"
	"github.com/xuando/gorouter/internal/oauth/providers"
)

// tokenExpiryBuffer is how far ahead of actual expiry we refresh (5 minutes).
const tokenExpiryBuffer = 5 * time.Minute

// RefreshResult holds refreshed token data.
type RefreshResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

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

// RefreshToken refreshes the access token for a given provider connection.
// Returns the new tokens or an error. The caller should persist refreshed tokens.
func RefreshToken(provider, refreshToken string) (*RefreshResult, error) {
	if refreshToken == "" {
		return nil, fmt.Errorf("no refresh token for provider %s", provider)
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

	default:
		return nil, fmt.Errorf("unsupported provider for token refresh: %s", provider)
	}
}

// EnsureFreshToken checks if a connection's token is expired and refreshes if needed.
// Updates the DB if a refresh occurs. Returns the (possibly refreshed) access token.
func EnsureFreshToken(conn *db.ProviderConnection, store *db.Store) (string, error) {
	if conn.AuthType != "oauth" || conn.RefreshToken == "" {
		return conn.AccessToken, nil
	}

	if !IsTokenExpired(conn.ExpiresAt) {
		return conn.AccessToken, nil
	}

	slog.Info("refreshing expired token", "provider", conn.Provider, "connID", conn.ID)

	result, err := RefreshToken(conn.Provider, conn.RefreshToken)
	if err != nil {
		slog.Error("token refresh failed", "provider", conn.Provider, "error", err)
		return conn.AccessToken, err // return old token, caller can decide to fail
	}

	var expiresAt string
	if result.ExpiresIn > 0 {
		expiresAt = time.Now().UTC().Add(time.Duration(result.ExpiresIn) * time.Second).Format(time.RFC3339)
	}

	updates := map[string]interface{}{
		"accessToken":  result.AccessToken,
		"refreshToken": result.RefreshToken,
		"expiresAt":    expiresAt,
	}
	if err := store.UpdateProviderConnection(conn.ID, updates); err != nil {
		slog.Error("failed to persist refreshed token", "error", err)
	}

	slog.Info("token refreshed successfully", "provider", conn.Provider)
	return result.AccessToken, nil
}

func orDefault(val, fallback string) string {
	if val != "" {
		return val
	}
	return fallback
}
