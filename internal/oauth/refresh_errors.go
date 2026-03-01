package oauth

import (
	"errors"
	"strings"
)

// Sentinel errors for token refresh classification.
var (
	// ErrTokenRevoked indicates the refresh token is permanently invalid
	// (e.g. user revoked access, or "invalid_grant" response).
	// Do NOT retry — mark connection inactive.
	ErrTokenRevoked = errors.New("token revoked or invalid_grant")

	// ErrRefreshUnsupported indicates the provider doesn't support token refresh.
	ErrRefreshUnsupported = errors.New("provider does not support token refresh")

	// ErrNoRefreshToken indicates no refresh token is available.
	ErrNoRefreshToken = errors.New("no refresh token available")
)

// IsTransientError returns true if the error is likely temporary
// and the refresh should be retried (network errors, 500, 502, 503).
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}
	// Permanent errors — never retry
	if errors.Is(err, ErrTokenRevoked) || errors.Is(err, ErrRefreshUnsupported) || errors.Is(err, ErrNoRefreshToken) {
		return false
	}
	msg := err.Error()
	// invalid_grant = token revoked
	if strings.Contains(msg, "invalid_grant") {
		return false
	}
	// HTTP 400/401/403 from token endpoint = permanent
	if strings.Contains(msg, "(400)") || strings.Contains(msg, "(401)") || strings.Contains(msg, "(403)") {
		return false
	}
	// Everything else (network, 500, 502, 503, 429, timeout) = transient
	return true
}

// ClassifyRefreshError wraps a raw provider error with the appropriate sentinel.
// Returns the original error if it's transient.
func ClassifyRefreshError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "invalid_grant") ||
		strings.Contains(msg, "(400)") ||
		strings.Contains(msg, "(401)") ||
		strings.Contains(msg, "(403)") {
		return ErrTokenRevoked
	}
	return err
}
