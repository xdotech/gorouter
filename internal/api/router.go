package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/xdotech/gorouter/internal/auth"
	"github.com/xdotech/gorouter/internal/config"
	"github.com/xdotech/gorouter/internal/db"
	"github.com/xdotech/gorouter/internal/oauth"
	"github.com/xdotech/gorouter/internal/usage"
)

// NewRouter builds and returns a chi.Router mounted at /api.
func NewRouter(store *db.Store, cfg *config.Config, usageDB *usage.DB) chi.Router {
	r := chi.NewRouter()

	authH := NewAuthHandler(store, cfg)
	settingsH := NewSettingsHandler(store, cfg)
	providersH := NewProvidersHandler(store)
	nodesH := NewProviderNodesHandler(store)
	combosH := NewCombosHandler(store)
	keysH := NewKeysHandler(store, cfg)
	aliasesH := NewAliasesHandler(store)
	usageH := NewUsageHandler(usageDB, usage.Global)
	pricingH := NewPricingHandler(store)

	getSettings := func() (bool, error) {
		s, err := store.GetSettings()
		if err != nil {
			return true, err
		}
		return s.RequireLogin, nil
	}
	loginRequired := auth.RequireLogin(cfg.JWTSecret, getSettings)

	// OAuth routes (public — callback must be accessible without login)
	r.Mount("/oauth", oauth.Routes(store, cfg))

	// Public routes (no auth required)
	r.Post("/auth/login", authH.Login)
	r.Post("/auth/logout", authH.Logout)
	r.Get("/init", settingsH.Init)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(loginRequired)

		// Settings
		r.Get("/settings", settingsH.GetSettings)
		r.Put("/settings", settingsH.UpdateSettings)
		r.Post("/settings/require-login", settingsH.RequireLogin)

		// Shutdown (simple stub)
		r.Post("/shutdown", func(w http.ResponseWriter, r *http.Request) {
			JSON(w, http.StatusOK, map[string]bool{"ok": true})
		})

		// Providers
		r.Get("/providers", providersH.List)
		r.Post("/providers/validate", providersH.Validate)
		r.Post("/providers", providersH.Create)
		r.Get("/providers/{id}", providersH.Get)
		r.Put("/providers/{id}", providersH.Update)
		r.Delete("/providers/{id}", providersH.Delete)
		r.Post("/providers/{id}/test", providersH.Test)
		r.Get("/providers/{id}/models", providersH.Models)

		// Provider nodes
		r.Get("/provider-nodes", nodesH.List)
		r.Post("/provider-nodes", nodesH.Create)
		r.Put("/provider-nodes/{id}", nodesH.Update)
		r.Delete("/provider-nodes/{id}", nodesH.Delete)

		// Combos
		r.Get("/combos", combosH.List)
		r.Post("/combos", combosH.Create)
		r.Put("/combos/{id}", combosH.Update)
		r.Delete("/combos/{id}", combosH.Delete)

		// API Keys
		r.Get("/keys", keysH.List)
		r.Post("/keys", keysH.Create)
		r.Delete("/keys/{id}", keysH.Delete)

		// Model aliases
		r.Get("/models/alias", aliasesH.List)
		r.Post("/models/alias", aliasesH.Set)
		r.Delete("/models/alias/{alias}", aliasesH.Delete)

		// Usage
		r.Get("/usage/providers", usageH.Providers)
		r.Get("/usage/request-logs", usageH.RequestLogs)
		r.Get("/usage/stream", usageH.Stream)

		// Pricing
		r.Get("/pricing", pricingH.Get)
		r.Put("/pricing", pricingH.Update)
	})

	return r
}
