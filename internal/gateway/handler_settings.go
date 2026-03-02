package gateway

import "net/http"

func (s *Server) handleGetSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st, err := s.stores.Settings.Get()
		if err != nil {
			renderError(w, http.StatusInternalServerError, "failed to load settings")
			return
		}
		st.PasswordHash = ""
		renderJSON(w, http.StatusOK, st)
	}
}

func (s *Server) handleUpdateSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var updates map[string]interface{}
		if err := decodeBody(r, &updates); err != nil {
			renderError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		delete(updates, "passwordHash")

		if err := s.stores.Settings.Update(updates); err != nil {
			renderError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}

		st, err := s.stores.Settings.Get()
		if err != nil {
			renderError(w, http.StatusInternalServerError, "failed to reload settings")
			return
		}
		st.PasswordHash = ""
		renderJSON(w, http.StatusOK, st)
	}
}

func (s *Server) handleRequireLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			RequireLogin bool `json:"requireLogin"`
		}
		if err := decodeBody(r, &body); err != nil {
			renderError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		updates := map[string]interface{}{"requireLogin": body.RequireLogin}
		if err := s.stores.Settings.Update(updates); err != nil {
			renderError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}
		renderJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func (s *Server) handleInit() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st, err := s.stores.Settings.Get()
		if err != nil {
			renderError(w, http.StatusInternalServerError, "failed to load settings")
			return
		}
		renderJSON(w, http.StatusOK, map[string]interface{}{
			"initialized":  true,
			"requireLogin": st.RequireLogin,
		})
	}
}

func (s *Server) handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
