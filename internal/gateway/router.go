package gateway

import (
	"net/http"
	"time"

	"github.com/xdotech/gorouter/dashboard"
	"github.com/xdotech/gorouter/internal/oauth"
)

func (s *Server) setupRouter() http.Handler {
	mux := http.NewServeMux()

	// ─── Health ──────────────────────────────────────────────────────────
	mux.HandleFunc("GET /health", s.handleHealth())

	// ─── OAuth routes (public) ───────────────────────────────────────────
	mux.HandleFunc("GET /api/oauth/{provider}/authorize", s.handleOAuthAuthorize())
	mux.HandleFunc("GET /api/oauth/{provider}/callback", s.handleOAuthCallback())
	mux.HandleFunc("POST /api/oauth/{provider}/device-code", s.handleDeviceCode())
	mux.HandleFunc("POST /api/oauth/{provider}/poll", s.handlePoll())

	// Codex/OpenAI: on-demand proxy on port 1455 (OpenAI only allows localhost:1455)
	mux.HandleFunc("GET /api/oauth/cx/authorize", s.handleCodexAuthorize())
	mux.HandleFunc("GET /api/oauth/cx/authcallback", s.handleCodexCallback())

	// Generic /callback for providers that use http://localhost:{port}/callback
	// (e.g. Claude Code). Looks up provider from OAuth state.
	mux.HandleFunc("GET /callback", func(w http.ResponseWriter, r *http.Request) {
		stateID := r.URL.Query().Get("state")
		if st, ok := oauth.GetState(stateID); ok {
			r.SetPathValue("provider", st.Provider)
		}
		s.oh.Callback(w, r)
	})

	// ─── Public auth routes ──────────────────────────────────────────────
	mux.HandleFunc("POST /api/auth/login", s.handleLogin())
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout())
	mux.HandleFunc("GET /api/init", s.handleInit())

	// ─── Protected management API ────────────────────────────────────────
	protect := s.loginRequired

	// Settings
	mux.Handle("GET /api/settings", protect(s.handleGetSettings()))
	mux.Handle("PUT /api/settings", protect(s.handleUpdateSettings()))
	mux.Handle("POST /api/settings/require-login", protect(s.handleRequireLogin()))

	// Shutdown stub
	mux.Handle("POST /api/shutdown", protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		renderJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})))

	// Providers
	mux.Handle("GET /api/providers", protect(s.handleListProviders()))
	mux.Handle("POST /api/providers/validate", protect(s.handleValidateProvider()))
	mux.Handle("POST /api/providers", protect(s.handleCreateProvider()))
	mux.Handle("GET /api/providers/{id}", protect(s.handleGetProvider()))
	mux.Handle("PUT /api/providers/{id}", protect(s.handleUpdateProvider()))
	mux.Handle("DELETE /api/providers/{id}", protect(s.handleDeleteProvider()))
	mux.Handle("POST /api/providers/{id}/test", protect(s.handleTestProvider()))
	mux.Handle("GET /api/providers/{id}/models", protect(s.handleProviderModels()))

	// Provider nodes
	mux.Handle("GET /api/provider-nodes", protect(s.handleListNodes()))
	mux.Handle("POST /api/provider-nodes", protect(s.handleCreateNode()))
	mux.Handle("PUT /api/provider-nodes/{id}", protect(s.handleUpdateNode()))
	mux.Handle("DELETE /api/provider-nodes/{id}", protect(s.handleDeleteNode()))

	// Combos
	mux.Handle("GET /api/combos", protect(s.handleListCombos()))
	mux.Handle("POST /api/combos", protect(s.handleCreateCombo()))
	mux.Handle("PUT /api/combos/{id}", protect(s.handleUpdateCombo()))
	mux.Handle("DELETE /api/combos/{id}", protect(s.handleDeleteCombo()))

	// API Keys
	mux.Handle("GET /api/keys", protect(s.handleListKeys()))
	mux.Handle("POST /api/keys", protect(s.handleCreateKey()))
	mux.Handle("DELETE /api/keys/{id}", protect(s.handleDeleteKey()))

	// Model aliases
	mux.Handle("GET /api/models/alias", protect(s.handleListAliases()))
	mux.Handle("POST /api/models/alias", protect(s.handleSetAlias()))
	mux.Handle("DELETE /api/models/alias/{alias}", protect(s.handleDeleteAlias()))

	// Usage
	mux.Handle("GET /api/usage/providers", protect(s.handleUsageProviders()))
	mux.Handle("GET /api/usage/request-logs", protect(s.handleUsageRequestLogs()))
	mux.Handle("GET /api/usage/stream", protect(s.handleUsageStream()))

	// Pricing
	mux.Handle("GET /api/pricing", protect(s.handleGetPricing()))
	mux.Handle("PUT /api/pricing", protect(s.handleUpdatePricing()))

	// ─── AI routing (v1) ─────────────────────────────────────────────────
	mux.Handle("POST /v1/chat/completions", s.apiKeyRequired(s.handleChat()))
	mux.Handle("GET /v1/models", s.apiKeyRequired(s.handleModels()))
	// Catch-all for other v1 routes
	mux.Handle("POST /v1/", s.apiKeyRequired(s.handleChat()))

	// ─── Dashboard ───────────────────────────────────────────────────────
	mux.Handle("/dashboard/", http.StripPrefix("/dashboard", dashboard.Handler()))
	mux.Handle("/", dashboard.Handler())

	// ─── Middleware stack ─────────────────────────────────────────────────
	var handler http.Handler = mux
	handler = http.TimeoutHandler(handler, 300*time.Second, "request timeout")
	handler = corsMiddleware(s.cfg)(handler)
	handler = recovererMiddleware()(handler)
	handler = loggerMiddleware()(handler)
	handler = requestIDMiddleware()(handler)

	return handler
}
