package auth

import (
	"net/http"
)

// RequireLogin is a chi-compatible middleware that enforces cookie-based JWT auth.
// If the getSettings func returns requireLogin=false, the check is skipped.
func RequireLogin(secret string, getSettings func() (requireLogin bool, err error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requireLogin, err := getSettings()
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if !requireLogin {
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie("auth_token")
			if err != nil || cookie.Value == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			valid, err := ValidateToken(cookie.Value, secret)
			if err != nil || !valid {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAPIKey is a chi-compatible middleware that enforces API key authentication.
func RequireAPIKey(store APIKeyValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := ExtractAPIKey(r)
			if !IsValidAPIKey(key, store) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
