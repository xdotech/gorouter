package api

import (
	"net/http"

	"github.com/xdotech/gorouter/internal/db"
)

// PricingHandler handles pricing config.
type PricingHandler struct {
	store *db.Store
}

func NewPricingHandler(store *db.Store) *PricingHandler {
	return &PricingHandler{store: store}
}

// Get GET /api/pricing
func (h *PricingHandler) Get(w http.ResponseWriter, r *http.Request) {
	pricing, err := h.store.GetPricing()
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to load pricing")
		return
	}
	JSON(w, http.StatusOK, pricing)
}

// Update PUT /api/pricing
// Body: map[string]float64
func (h *PricingHandler) Update(w http.ResponseWriter, r *http.Request) {
	var pricing map[string]float64
	if err := DecodeBody(r, &pricing); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.store.UpdatePricing(pricing); err != nil {
		Error(w, http.StatusInternalServerError, "failed to update pricing")
		return
	}
	JSON(w, http.StatusOK, pricing)
}
