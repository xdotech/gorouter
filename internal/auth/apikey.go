package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
)

// GenerateAPIKey produces an HMAC-SHA256 signed API key.
// Format: sk_9r_<base64url(keyId + hmac_bytes)>
func GenerateAPIKey(keyID, machineID, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(keyID + machineID))
	hmacBytes := mac.Sum(nil)

	// Concatenate keyID bytes + hmac bytes, then base64url-encode (no padding)
	raw := append([]byte(keyID), hmacBytes...)
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	return "sk_9r_" + encoded
}

// ExtractAPIKey retrieves the API key from Authorization: Bearer or x-api-key header.
func ExtractAPIKey(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
	}
	return r.Header.Get("x-api-key")
}

// APIKeyValidator is the minimal interface required to validate an API key.
type APIKeyValidator interface {
	ValidateAPIKey(string) (bool, error)
}

// IsValidAPIKey checks whether the given key is valid via the store.
func IsValidAPIKey(key string, store APIKeyValidator) bool {
	if key == "" {
		return false
	}
	ok, err := store.ValidateAPIKey(key)
	if err != nil {
		return false
	}
	return ok
}
