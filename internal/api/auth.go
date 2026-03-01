package api

import (
	"net/http"
	"time"

	"github.com/xdotech/gorouter/internal/auth"
	"github.com/xdotech/gorouter/internal/config"
	"github.com/xdotech/gorouter/internal/db"
)

// AuthHandler handles login and logout.
type AuthHandler struct {
	store *db.Store
	cfg   *config.Config
}

func NewAuthHandler(store *db.Store, cfg *config.Config) *AuthHandler {
	return &AuthHandler{store: store, cfg: cfg}
}

// Login POST /api/auth/login
// Body: {"password": "..."}
// Sets auth_token cookie (30-day JWT) and returns {"ok": true}.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := DecodeBody(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Password == "" {
		Error(w, http.StatusBadRequest, "password required")
		return
	}

	settings, err := h.store.GetSettings()
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to load settings")
		return
	}

	var valid bool
	if settings.PasswordHash != "" {
		valid = auth.VerifyPassword(body.Password, settings.PasswordHash)
	} else {
		valid = auth.VerifyInitialPassword(body.Password, h.cfg.InitialPassword)
	}

	if !valid {
		Error(w, http.StatusUnauthorized, "invalid password")
		return
	}

	token, err := auth.GenerateToken(h.cfg.JWTSecret)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().Add(30 * 24 * time.Hour),
		SameSite: http.SameSiteLaxMode,
	})
	JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Logout POST /api/auth/logout
// Clears auth_token cookie and returns {"ok": true}.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
	JSON(w, http.StatusOK, map[string]bool{"ok": true})
}
