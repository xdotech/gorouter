package api

import (
	"net/http"

	"github.com/xdotech/gorouter/internal/config"
	"github.com/xdotech/gorouter/internal/db"
)

// SettingsHandler handles settings and init routes.
type SettingsHandler struct {
	store *db.Store
	cfg   *config.Config
}

func NewSettingsHandler(store *db.Store, cfg *config.Config) *SettingsHandler {
	return &SettingsHandler{store: store, cfg: cfg}
}

// GetSettings GET /api/settings
func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	s, err := h.store.GetSettings()
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to load settings")
		return
	}
	s.PasswordHash = "" // omit sensitive field
	JSON(w, http.StatusOK, s)
}

// UpdateSettings PUT /api/settings
func (h *SettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var updates map[string]interface{}
	if err := DecodeBody(r, &updates); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	// Never allow direct passwordHash update through this endpoint
	delete(updates, "passwordHash")

	if err := h.store.UpdateSettings(updates); err != nil {
		Error(w, http.StatusInternalServerError, "failed to update settings")
		return
	}

	s, err := h.store.GetSettings()
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to reload settings")
		return
	}
	s.PasswordHash = ""
	JSON(w, http.StatusOK, s)
}

// RequireLogin POST /api/settings/require-login
// Body: {"requireLogin": bool}
func (h *SettingsHandler) RequireLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RequireLogin bool `json:"requireLogin"`
	}
	if err := DecodeBody(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	updates := map[string]interface{}{"requireLogin": body.RequireLogin}
	if err := h.store.UpdateSettings(updates); err != nil {
		Error(w, http.StatusInternalServerError, "failed to update settings")
		return
	}
	JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Init GET /api/init
// Returns initialization status.
func (h *SettingsHandler) Init(w http.ResponseWriter, r *http.Request) {
	s, err := h.store.GetSettings()
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to load settings")
		return
	}
	JSON(w, http.StatusOK, map[string]interface{}{
		"initialized":  true,
		"requireLogin": s.RequireLogin,
	})
}
