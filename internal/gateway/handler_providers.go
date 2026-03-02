package gateway

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/xdotech/gorouter/internal/domain"
)

func maskConnection(c domain.ProviderConnection) domain.ProviderConnection {
	if len(c.APIKey) > 4 {
		c.APIKey = c.APIKey[:4] + "..."
	}
	c.AccessToken = ""
	c.RefreshToken = ""
	return c
}

func (s *Server) handleListProviders() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conns, err := s.stores.Connections.List(domain.ConnectionFilter{})
		if err != nil {
			renderError(w, http.StatusInternalServerError, "failed to list providers")
			return
		}
		out := make([]domain.ProviderConnection, len(conns))
		for i, c := range conns {
			out[i] = maskConnection(c)
		}
		renderJSON(w, http.StatusOK, out)
	}
}

func (s *Server) handleCreateProvider() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var conn domain.ProviderConnection
		if err := decodeBody(r, &conn); err != nil {
			renderError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		conn.ID = uuid.New().String()
		conn.AuthType = "apikey"
		conn.TestStatus = "unknown"
		if err := s.stores.Connections.Create(conn); err != nil {
			renderError(w, http.StatusInternalServerError, "failed to create provider")
			return
		}
		renderJSON(w, http.StatusCreated, maskConnection(conn))
	}
}

func (s *Server) handleGetProvider() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		conn, err := s.stores.Connections.Get(id)
		if err != nil {
			renderError(w, http.StatusNotFound, "provider not found")
			return
		}
		renderJSON(w, http.StatusOK, maskConnection(*conn))
	}
}

func (s *Server) handleUpdateProvider() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var updates map[string]interface{}
		if err := decodeBody(r, &updates); err != nil {
			renderError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := s.stores.Connections.Update(id, updates); err != nil {
			renderError(w, http.StatusNotFound, "provider not found")
			return
		}
		conn, err := s.stores.Connections.Get(id)
		if err != nil {
			renderError(w, http.StatusInternalServerError, "failed to reload provider")
			return
		}
		renderJSON(w, http.StatusOK, maskConnection(*conn))
	}
}

func (s *Server) handleDeleteProvider() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := s.stores.Connections.Delete(id); err != nil {
			renderError(w, http.StatusNotFound, "provider not found")
			return
		}
		// Also delete from legacy db.Store so the refresh scheduler's
		// in-memory copy doesn't re-add the connection on next save.
		_ = s.dbStore.DeleteProviderConnection(id)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) handleTestProvider() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderJSON(w, http.StatusOK, map[string]string{"ok": "true", "status": "active"})
	}
}

func (s *Server) handleProviderModels() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderJSON(w, http.StatusOK, []interface{}{})
	}
}

func (s *Server) handleValidateProvider() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var conn domain.ProviderConnection
		if err := decodeBody(r, &conn); err != nil {
			renderError(w, http.StatusBadRequest, "invalid request body")
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
			renderJSON(w, http.StatusBadRequest, map[string]interface{}{"errors": errs})
			return
		}
		renderJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}
