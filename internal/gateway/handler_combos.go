package gateway

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/xdotech/gorouter/internal/domain"
)

func (s *Server) handleListCombos() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		combos, err := s.stores.Combos.List()
		if err != nil {
			renderError(w, http.StatusInternalServerError, "failed to list combos")
			return
		}
		renderJSON(w, http.StatusOK, combos)
	}
}

func (s *Server) handleCreateCombo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var combo domain.Combo
		if err := decodeBody(r, &combo); err != nil {
			renderError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if combo.Name == "" {
			renderError(w, http.StatusBadRequest, "name required")
			return
		}
		combo.ID = uuid.New().String()
		if combo.Models == nil {
			combo.Models = []string{}
		}
		if err := s.stores.Combos.Create(combo); err != nil {
			renderError(w, http.StatusInternalServerError, "failed to create combo")
			return
		}
		renderJSON(w, http.StatusCreated, combo)
	}
}

func (s *Server) handleUpdateCombo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var combo domain.Combo
		if err := decodeBody(r, &combo); err != nil {
			renderError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		combo.ID = id
		if combo.Models == nil {
			combo.Models = []string{}
		}
		if err := s.stores.Combos.Update(id, combo); err != nil {
			renderError(w, http.StatusNotFound, "combo not found")
			return
		}
		renderJSON(w, http.StatusOK, combo)
	}
}

func (s *Server) handleDeleteCombo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := s.stores.Combos.Delete(id); err != nil {
			renderError(w, http.StatusNotFound, "combo not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
