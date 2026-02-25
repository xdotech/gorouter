package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// PKCEChallenge holds PKCE code verifier and challenge for OAuth flows.
type PKCEChallenge struct {
	Verifier  string
	Challenge string // base64url(sha256(verifier)), no padding
	Method    string // "S256"
}

// GeneratePKCE generates a new PKCE code verifier and challenge pair.
func GeneratePKCE() (*PKCEChallenge, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generate pkce random bytes: %w", err)
	}

	verifier := base64.RawURLEncoding.EncodeToString(raw)

	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	return &PKCEChallenge{
		Verifier:  verifier,
		Challenge: challenge,
		Method:    "S256",
	}, nil
}
