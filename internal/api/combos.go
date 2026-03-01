package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/xdotech/gorouter/internal/db"
)

// CombosHandler handles combo CRUD.
type CombosHandler struct {
	store *db.Store
}

func NewCombosHandler(store *db.Store) *CombosHandler {
	return &CombosHandler{store: store}
}

// List GET /api/combos
func (h *CombosHandler) List(w http.ResponseWriter, r *http.Request) {
	combos, err := h.store.GetCombos()
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to list combos")
		return
	}
	JSON(w, http.StatusOK, combos)
}

// Create POST /api/combos
func (h *CombosHandler) Create(w http.ResponseWriter, r *http.Request) {
	var combo db.Combo
	if err := DecodeBody(r, &combo); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if combo.Name == "" {
		Error(w, http.StatusBadRequest, "name required")
		return
	}
	combo.ID = uuid.New().String()
	if combo.Models == nil {
		combo.Models = []string{}
	}
	if err := h.store.CreateCombo(combo); err != nil {
		Error(w, http.StatusInternalServerError, "failed to create combo")
		return
	}
	JSON(w, http.StatusCreated, combo)
}

// Update PUT /api/combos/{id}
func (h *CombosHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var combo db.Combo
	if err := DecodeBody(r, &combo); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	combo.ID = id
	if combo.Models == nil {
		combo.Models = []string{}
	}
	if err := h.store.UpdateCombo(id, combo); err != nil {
		Error(w, http.StatusNotFound, "combo not found")
		return
	}
	JSON(w, http.StatusOK, combo)
}

// Delete DELETE /api/combos/{id}
func (h *CombosHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteCombo(id); err != nil {
		Error(w, http.StatusNotFound, "combo not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
