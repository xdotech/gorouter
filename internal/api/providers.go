package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/xdotech/gorouter/internal/db"
)

// ProvidersHandler handles provider connection CRUD.
type ProvidersHandler struct {
	store *db.Store
}

func NewProvidersHandler(store *db.Store) *ProvidersHandler {
	return &ProvidersHandler{store: store}
}

func maskConnection(c db.ProviderConnection) db.ProviderConnection {
	if len(c.APIKey) > 4 {
		c.APIKey = c.APIKey[:4] + "..."
	}
	c.AccessToken = ""
	c.RefreshToken = ""
	return c
}

// List GET /api/providers
func (h *ProvidersHandler) List(w http.ResponseWriter, r *http.Request) {
	conns, err := h.store.GetProviderConnections(db.ConnectionFilter{})
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to list providers")
		return
	}
	out := make([]db.ProviderConnection, len(conns))
	for i, c := range conns {
		out[i] = maskConnection(c)
	}
	JSON(w, http.StatusOK, out)
}

// Create POST /api/providers
func (h *ProvidersHandler) Create(w http.ResponseWriter, r *http.Request) {
	var conn db.ProviderConnection
	if err := DecodeBody(r, &conn); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	conn.ID = uuid.New().String()
	conn.AuthType = "apikey"
	conn.TestStatus = "unknown"
	if err := h.store.CreateProviderConnection(conn); err != nil {
		Error(w, http.StatusInternalServerError, "failed to create provider")
		return
	}
	JSON(w, http.StatusCreated, maskConnection(conn))
}

// Get GET /api/providers/{id}
func (h *ProvidersHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	conn, err := h.store.GetProviderConnection(id)
	if err != nil {
		Error(w, http.StatusNotFound, "provider not found")
		return
	}
	JSON(w, http.StatusOK, maskConnection(*conn))
}

// Update PUT /api/providers/{id}
func (h *ProvidersHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var updates map[string]interface{}
	if err := DecodeBody(r, &updates); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.store.UpdateProviderConnection(id, updates); err != nil {
		Error(w, http.StatusNotFound, "provider not found")
		return
	}
	conn, err := h.store.GetProviderConnection(id)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to reload provider")
		return
	}
	JSON(w, http.StatusOK, maskConnection(*conn))
}

// Delete DELETE /api/providers/{id}
func (h *ProvidersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteProviderConnection(id); err != nil {
		Error(w, http.StatusNotFound, "provider not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Test POST /api/providers/{id}/test
func (h *ProvidersHandler) Test(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, map[string]string{"ok": "true", "status": "active"})
}

// Models GET /api/providers/{id}/models
func (h *ProvidersHandler) Models(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, []interface{}{})
}

// Validate POST /api/providers/validate
func (h *ProvidersHandler) Validate(w http.ResponseWriter, r *http.Request) {
	var conn db.ProviderConnection
	if err := DecodeBody(r, &conn); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	errs := map[string]string{}
	if conn.Provider == "" {
		errs["provider"] = "required"
	}
	if conn.APIKey == "" {
		errs["apiKey"] = "required"
	}
	if len(errs) > 0 {
		JSON(w, http.StatusBadRequest, map[string]interface{}{"errors": errs})
		return
	}
	JSON(w, http.StatusOK, map[string]bool{"ok": true})
}
