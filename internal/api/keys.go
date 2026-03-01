package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/xdotech/gorouter/internal/auth"
	"github.com/xdotech/gorouter/internal/config"
	"github.com/xdotech/gorouter/internal/db"
)

// KeysHandler manages API key generation and deletion.
type KeysHandler struct {
	store *db.Store
	cfg   *config.Config
}

func NewKeysHandler(store *db.Store, cfg *config.Config) *KeysHandler {
	return &KeysHandler{store: store, cfg: cfg}
}

func maskKey(k db.APIKey) db.APIKey {
	if len(k.Key) > 4 {
		k.Key = k.Key[:4] + "..."
	}
	return k
}

// List GET /api/keys
func (h *KeysHandler) List(w http.ResponseWriter, r *http.Request) {
	keys, err := h.store.GetAPIKeys()
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to list keys")
		return
	}
	out := make([]db.APIKey, len(keys))
	for i, k := range keys {
		out[i] = maskKey(k)
	}
	JSON(w, http.StatusOK, out)
}

// Create POST /api/keys
func (h *KeysHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := DecodeBody(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" {
		body.Name = "API Key"
	}

	keyID := uuid.New().String()
	machineID := auth.GetMachineID(h.cfg.MachineIDSalt, h.cfg.DataDir)
	keyValue := auth.GenerateAPIKey(keyID, machineID, h.cfg.APIKeySecret)

	apiKey := db.APIKey{
		ID:        keyID,
		Name:      body.Name,
		Key:       keyValue,
		MachineID: machineID,
		IsActive:  true,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := h.store.CreateAPIKey(apiKey); err != nil {
		Error(w, http.StatusInternalServerError, "failed to create key")
		return
	}
	// Return full key on creation only
	JSON(w, http.StatusCreated, apiKey)
}

// Delete DELETE /api/keys/{id}
func (h *KeysHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteAPIKey(id); err != nil {
		Error(w, http.StatusNotFound, "key not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
