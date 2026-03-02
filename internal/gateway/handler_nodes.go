package gateway

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/xdotech/gorouter/internal/domain"
)

func (s *Server) handleListNodes() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodes, err := s.stores.Nodes.List()
		if err != nil {
			renderError(w, http.StatusInternalServerError, "failed to list nodes")
			return
		}
		renderJSON(w, http.StatusOK, nodes)
	}
}

func (s *Server) handleCreateNode() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var node domain.ProviderNode
		if err := decodeBody(r, &node); err != nil {
			renderError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		node.ID = uuid.New().String()
		if err := s.stores.Nodes.Create(node); err != nil {
			renderError(w, http.StatusInternalServerError, "failed to create node")
			return
		}
		renderJSON(w, http.StatusCreated, node)
	}
}

func (s *Server) handleUpdateNode() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var node domain.ProviderNode
		if err := decodeBody(r, &node); err != nil {
			renderError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		node.ID = id
		if err := s.stores.Nodes.Update(id, node); err != nil {
			renderError(w, http.StatusNotFound, "node not found")
			return
		}
		renderJSON(w, http.StatusOK, node)
	}
}

func (s *Server) handleDeleteNode() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := s.stores.Nodes.Delete(id); err != nil {
			renderError(w, http.StatusNotFound, "node not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
