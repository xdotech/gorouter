package gateway

import "net/http"

func (s *Server) handleGetPricing() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pricing, err := s.stores.Pricing.Get()
		if err != nil {
			renderError(w, http.StatusInternalServerError, "failed to load pricing")
			return
		}
		renderJSON(w, http.StatusOK, pricing)
	}
}

func (s *Server) handleUpdatePricing() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var pricing map[string]float64
		if err := decodeBody(r, &pricing); err != nil {
			renderError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := s.stores.Pricing.Update(pricing); err != nil {
			renderError(w, http.StatusInternalServerError, "failed to update pricing")
			return
		}
		renderJSON(w, http.StatusOK, pricing)
	}
}
