package oauth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// OAuthState holds temporary state for an in-progress OAuth flow.
type OAuthState struct {
	Provider     string
	PKCEVerifier string
	CreatedAt    time.Time
	Extra        map[string]string // for provider-specific data
}

var states sync.Map // key: stateID → *OAuthState

// StoreState stores an OAuth state entry and returns the state ID.
// Cleans up states older than 10 minutes on each call.
func StoreState(provider string, pkce *PKCEChallenge) string {
	cleanupExpiredStates()

	stateID := generateStateID()
	verifier := ""
	if pkce != nil {
		verifier = pkce.Verifier
	}

	states.Store(stateID, &OAuthState{
		Provider:     provider,
		PKCEVerifier: verifier,
		CreatedAt:    time.Now(),
		Extra:        make(map[string]string),
	})
	return stateID
}

// GetState retrieves a stored OAuth state by ID.
func GetState(stateID string) (*OAuthState, bool) {
	v, ok := states.Load(stateID)
	if !ok {
		return nil, false
	}
	s, ok := v.(*OAuthState)
	return s, ok
}

// DeleteState removes a stored OAuth state by ID.
func DeleteState(stateID string) {
	states.Delete(stateID)
}

func generateStateID() string {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		// fallback using timestamp nanos as hex — should never happen
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func cleanupExpiredStates() {
	cutoff := time.Now().Add(-10 * time.Minute)
	states.Range(func(key, value any) bool {
		if s, ok := value.(*OAuthState); ok && s.CreatedAt.Before(cutoff) {
			states.Delete(key)
		}
		return true
	})
}
