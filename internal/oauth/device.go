package oauth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/xdotech/gorouter/internal/db"
	"github.com/xdotech/gorouter/internal/oauth/providers"
)

// DeviceCode initiates a device code flow for supported providers (qw).
func (h *Handler) DeviceCode(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")

	switch provider {
	case "qw":
		h.qwenDeviceCode(w, r)
	default:
		jsonError(w, fmt.Sprintf("device code not supported for provider: %s", provider), http.StatusBadRequest)
	}
}

// Poll polls a device code flow for token completion.
func (h *Handler) Poll(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")

	switch provider {
	case "qw":
		h.qwenPoll(w, r)
	default:
		jsonError(w, fmt.Sprintf("poll not supported for provider: %s", provider), http.StatusBadRequest)
	}
}

func (h *Handler) qwenDeviceCode(w http.ResponseWriter, _ *http.Request) {
	pkce, err := GeneratePKCE()
	if err != nil {
		jsonError(w, "failed to generate PKCE: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := providers.StartQwenDeviceCode(pkce.Challenge)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}

	// Store PKCE verifier keyed by device_code for later poll
	states.Store("qw:"+data.DeviceCode, &OAuthState{
		Provider:     "qwen",
		PKCEVerifier: pkce.Verifier,
		CreatedAt:    time.Now(),
		Extra:        make(map[string]string),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) qwenPoll(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceCode string `json:"device_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DeviceCode == "" {
		jsonError(w, "device_code required", http.StatusBadRequest)
		return
	}

	v, ok := states.Load("qw:" + req.DeviceCode)
	if !ok {
		jsonError(w, "unknown device_code", http.StatusBadRequest)
		return
	}
	oauthState := v.(*OAuthState)

	at, rt, err := providers.PollQwenDeviceCode(req.DeviceCode, oauthState.PKCEVerifier)
	if err != nil {
		errStr := err.Error()
		if errStr == "authorization_pending" || errStr == "slow_down" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]string{"status": errStr})
			return
		}
		jsonError(w, errStr, http.StatusBadGateway)
		return
	}

	states.Delete("qw:" + req.DeviceCode)

	conn := db.ProviderConnection{
		ID:           uuid.New().String(),
		Provider:     "qwen",
		AuthType:     "oauth",
		Name:         "Qwen",
		IsActive:     true,
		AccessToken:  at,
		RefreshToken: rt,
		TestStatus:   "unknown",
	}
	if err := h.store.CreateProviderConnection(conn); err != nil {
		jsonError(w, "failed to save connection: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "connected"})
}
