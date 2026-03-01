package oauth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xdotech/gorouter/internal/config"
	"github.com/xdotech/gorouter/internal/db"
	"github.com/xdotech/gorouter/internal/oauth/providers"
)

// Handler holds dependencies for OAuth HTTP handlers.
type Handler struct {
	store   *db.Store
	cfg     *config.Config
	baseURL string
}

// NewHandler creates a new OAuth Handler.
func NewHandler(store *db.Store, cfg *config.Config) *Handler {
	return &Handler{store: store, cfg: cfg, baseURL: cfg.BaseURL}
}

// Routes returns a chi.Router with all OAuth routes mounted.
func Routes(store *db.Store, cfg *config.Config) *http.ServeMux {
	h := NewHandler(store, cfg)
	r := http.NewServeMux()

	// PKCE authorize/callback (cc, gc, gh, if)
	r.HandleFunc("GET /api/oauth/{provider}/authorize", h.authorizeHandler)
	r.HandleFunc("GET /api/oauth/{provider}/callback", h.callbackHandler)

	// Device code flows (qw)
	r.HandleFunc("POST /api/oauth/{provider}/device-code", h.deviceCodeHandler)
	r.HandleFunc("POST /api/oauth/{provider}/poll", h.pollHandler)

	return r
}

func (h *Handler) authorizeHandler(w http.ResponseWriter, r *http.Request) {
	h.Authorize(w, r)
}

func (h *Handler) callbackHandler(w http.ResponseWriter, r *http.Request) {
	h.Callback(w, r)
}

func (h *Handler) deviceCodeHandler(w http.ResponseWriter, r *http.Request) {
	h.DeviceCode(w, r)
}

func (h *Handler) pollHandler(w http.ResponseWriter, r *http.Request) {
	h.Poll(w, r)
}

// Authorize redirects the user to the provider's OAuth authorization page.
func (h *Handler) Authorize(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")

	pkce, err := GeneratePKCE()
	if err != nil {
		http.Error(w, "failed to generate PKCE: "+err.Error(), http.StatusInternalServerError)
		return
	}

	stateID := StoreState(provider, pkce)
	authURL, err := h.buildAuthURL(provider, stateID, pkce.Challenge)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

// Callback handles the OAuth callback, exchanges code for tokens, and saves to DB.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	code := r.URL.Query().Get("code")
	stateID := r.URL.Query().Get("state")

	if code == "" {
		errMsg := r.URL.Query().Get("error_description")
		if errMsg == "" {
			errMsg = r.URL.Query().Get("error")
		}
		http.Error(w, "authorization error: "+errMsg, http.StatusBadRequest)
		return
	}

	oauthState, ok := GetState(stateID)
	if !ok {
		http.Error(w, "invalid or expired state", http.StatusBadRequest)
		return
	}
	DeleteState(stateID)

	conn, err := h.exchangeAndBuild(provider, code, oauthState)
	if err != nil {
		http.Error(w, "token exchange failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.store.CreateProviderConnection(*conn); err != nil {
		http.Error(w, "failed to save connection: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// ─── helpers ────────────────────────────────────────────────────────────────

func (h *Handler) buildAuthURL(provider, state, challenge string) (string, error) {
	switch provider {
	case "cc":
		return providers.BuildClaudeAuthURL(h.baseURL, state, challenge), nil
	case "gc":
		return providers.BuildGeminiAuthURL(h.baseURL, state, challenge), nil
	case "gh":
		return providers.BuildGitHubAuthURL(h.baseURL, state), nil
	case "if":
		return providers.BuildIFlowAuthURL(h.baseURL, state, challenge), nil
	case "cx":
		return providers.BuildCodexAuthURL(h.baseURL, state, challenge), nil
	case "ag":
		return providers.BuildAntigravityAuthURL(h.baseURL, state, challenge), nil
	default:
		return "", fmt.Errorf("unsupported provider: %s", provider)
	}
}

func (h *Handler) exchangeAndBuild(provider, code string, state *OAuthState) (*db.ProviderConnection, error) {
	conn := &db.ProviderConnection{
		ID:         uuid.New().String(),
		AuthType:   "oauth",
		IsActive:   true,
		TestStatus: "unknown",
	}

	switch provider {
	case "cc":
		redirectURI := strings.Replace(providers.ClaudeRedirectURI, "{baseURL}", h.baseURL, 1)
		tokens, err := providers.ExchangeClaudeCode(code, state.PKCEVerifier, redirectURI)
		if err != nil {
			return nil, err
		}
		conn.Provider = "claude-code"
		conn.Name = "Claude Code"
		conn.AccessToken = tokens.AccessToken
		conn.RefreshToken = tokens.RefreshToken
		if tokens.ExpiresIn > 0 {
			conn.ExpiresAt = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
		}

	case "gc":
		redirectURI := strings.Replace(providers.GeminiRedirectURI, "{baseURL}", h.baseURL, 1)
		tokens, err := providers.ExchangeGeminiCode(code, state.PKCEVerifier, redirectURI)
		if err != nil {
			return nil, err
		}
		conn.Provider = "gemini-cli"
		conn.Name = "Gemini CLI"
		conn.AccessToken = tokens.AccessToken
		conn.RefreshToken = tokens.RefreshToken
		if tokens.ExpiresIn > 0 {
			conn.ExpiresAt = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
		}

	case "gh":
		redirectURI := h.baseURL + "/api/oauth/gh/callback"
		ghTokens, err := providers.ExchangeGitHubCode(code, redirectURI)
		if err != nil {
			return nil, err
		}
		copilotToken, expiresAt, err := providers.FetchCopilotToken(ghTokens.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("fetch copilot token: %w", err)
		}
		conn.Provider = "github"
		conn.Name = "GitHub Copilot"
		conn.AccessToken = ghTokens.AccessToken
		conn.ExpiresAt = expiresAt
		conn.ProviderSpecificData = map[string]interface{}{
			"copilotToken": copilotToken,
			"expiresAt":    expiresAt,
		}

	case "if":
		redirectURI := strings.Replace(providers.IFlowRedirectURI, "{baseURL}", h.baseURL, 1)
		at, rt, err := providers.ExchangeIFlowCode(code, state.PKCEVerifier, redirectURI)
		if err != nil {
			return nil, err
		}
		conn.Provider = "iflow"
		conn.Name = "iFlow"
		conn.AccessToken = at
		conn.RefreshToken = rt

	case "cx":
		redirectURI := strings.Replace(providers.CodexRedirectURI, "{baseURL}", h.baseURL, 1)
		tokens, err := providers.ExchangeCodexCode(code, state.PKCEVerifier, redirectURI)
		if err != nil {
			return nil, err
		}
		conn.Provider = "codex"
		conn.Name = "OpenAI (Codex)"
		conn.AccessToken = tokens.AccessToken
		conn.RefreshToken = tokens.RefreshToken
		if tokens.ExpiresIn > 0 {
			conn.ExpiresAt = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
		}

	case "ag":
		redirectURI := strings.Replace(providers.AntigravityRedirectURI, "{baseURL}", h.baseURL, 1)
		tokens, err := providers.ExchangeAntigravityCode(code, state.PKCEVerifier, redirectURI)
		if err != nil {
			return nil, err
		}
		conn.Provider = "antigravity"
		conn.Name = "Antigravity"
		conn.AccessToken = tokens.AccessToken
		conn.RefreshToken = tokens.RefreshToken
		if tokens.ExpiresIn > 0 {
			conn.ExpiresAt = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
		}

	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	return conn, nil
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
