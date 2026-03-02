package gateway

import "net/http"

func (s *Server) handleListAliases() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		aliases, err := s.stores.Aliases.List()
		if err != nil {
			renderError(w, http.StatusInternalServerError, "failed to list aliases")
			return
		}
		renderJSON(w, http.StatusOK, aliases)
	}
}

func (s *Server) handleSetAlias() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Alias  string `json:"alias"`
			Target string `json:"target"`
		}
		if err := decodeBody(r, &body); err != nil {
			renderError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Alias == "" || body.Target == "" {
			renderError(w, http.StatusBadRequest, "alias and target required")
			return
		}
		if err := s.stores.Aliases.Set(body.Alias, body.Target); err != nil {
			renderError(w, http.StatusInternalServerError, "failed to save alias")
			return
		}
		renderJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func (s *Server) handleDeleteAlias() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		alias := r.PathValue("alias")
		if err := s.stores.Aliases.Delete(alias); err != nil {
			renderError(w, http.StatusInternalServerError, "failed to delete alias")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
