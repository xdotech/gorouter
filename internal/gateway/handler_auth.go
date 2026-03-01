package gateway

import (
	"net/http"
	"time"

	"github.com/xdotech/gorouter/internal/auth"
)

func (s *Server) handleLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Password string `json:"password"`
		}
		if err := decodeBody(r, &body); err != nil {
			renderError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Password == "" {
			renderError(w, http.StatusBadRequest, "password required")
			return
		}

		settings, err := s.stores.Settings.Get()
		if err != nil {
			renderError(w, http.StatusInternalServerError, "failed to load settings")
			return
		}

		var valid bool
		if settings.PasswordHash != "" {
			valid = auth.VerifyPassword(body.Password, settings.PasswordHash)
		} else {
			valid = auth.VerifyInitialPassword(body.Password, s.cfg.InitialPassword)
		}
		if !valid {
			renderError(w, http.StatusUnauthorized, "invalid password")
			return
		}

		token, err := auth.GenerateToken(s.cfg.JWTSecret)
		if err != nil {
			renderError(w, http.StatusInternalServerError, "failed to generate token")
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
		renderJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func (s *Server) handleLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     "auth_token",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
		})
		renderJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}
