package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/xuando/gorouter/internal/db"
)

// ProviderNodesHandler handles custom node CRUD.
type ProviderNodesHandler struct {
	store *db.Store
}

func NewProviderNodesHandler(store *db.Store) *ProviderNodesHandler {
	return &ProviderNodesHandler{store: store}
}

// List GET /api/provider-nodes
func (h *ProviderNodesHandler) List(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.store.GetProviderNodes()
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to list nodes")
		return
	}
	JSON(w, http.StatusOK, nodes)
}

// Create POST /api/provider-nodes
func (h *ProviderNodesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var node db.ProviderNode
	if err := DecodeBody(r, &node); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	node.ID = uuid.New().String()
	if err := h.store.CreateProviderNode(node); err != nil {
		Error(w, http.StatusInternalServerError, "failed to create node")
		return
	}
	JSON(w, http.StatusCreated, node)
}

// Update PUT /api/provider-nodes/{id}
func (h *ProviderNodesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var node db.ProviderNode
	if err := DecodeBody(r, &node); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	node.ID = id
	if err := h.store.UpdateProviderNode(id, node); err != nil {
		Error(w, http.StatusNotFound, "node not found")
		return
	}
	JSON(w, http.StatusOK, node)
}

// Delete DELETE /api/provider-nodes/{id}
func (h *ProviderNodesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteProviderNode(id); err != nil {
		Error(w, http.StatusNotFound, "node not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
