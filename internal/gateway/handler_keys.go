package gateway

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/xdotech/gorouter/internal/auth"
	"github.com/xdotech/gorouter/internal/domain"
)

func maskKey(k domain.APIKey) domain.APIKey {
	if len(k.Key) > 4 {
		k.Key = k.Key[:4] + "..."
	}
	return k
}

func (s *Server) handleListKeys() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keys, err := s.stores.APIKeys.List()
		if err != nil {
			renderError(w, http.StatusInternalServerError, "failed to list keys")
			return
		}
		out := make([]domain.APIKey, len(keys))
		for i, k := range keys {
			out[i] = maskKey(k)
		}
		renderJSON(w, http.StatusOK, out)
	}
}

func (s *Server) handleCreateKey() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name string `json:"name"`
		}
		if err := decodeBody(r, &body); err != nil {
			renderError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Name == "" {
			body.Name = "API Key"
		}

		keyID := uuid.New().String()
		machineID := auth.GetMachineID(s.cfg.MachineIDSalt, s.cfg.DataDir)
		keyValue := auth.GenerateAPIKey(keyID, machineID, s.cfg.APIKeySecret)

		apiKey := domain.APIKey{
			ID:        keyID,
			Name:      body.Name,
			Key:       keyValue,
			MachineID: machineID,
			IsActive:  true,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if err := s.stores.APIKeys.Create(apiKey); err != nil {
			renderError(w, http.StatusInternalServerError, "failed to create key")
			return
		}
		renderJSON(w, http.StatusCreated, apiKey)
	}
}

func (s *Server) handleDeleteKey() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := s.stores.APIKeys.Delete(id); err != nil {
			renderError(w, http.StatusNotFound, "key not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
