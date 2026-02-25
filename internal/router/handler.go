package router

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/xuando/gorouter/internal/auth"
	"github.com/xuando/gorouter/internal/db"
	"github.com/xuando/gorouter/internal/executor"
	"github.com/xuando/gorouter/internal/translator"
	"github.com/xuando/gorouter/internal/usage"
)

// Handler is the main routing handler wiring model resolution, account selection, and execution.
type Handler struct {
	store   *db.Store
	tracker *usage.Tracker
}

// NewHandler creates a new Handler.
func NewHandler(store *db.Store, tracker *usage.Tracker) *Handler {
	return &Handler{store: store, tracker: tracker}
}

// HandleChat handles POST /v1/chat/completions, /v1/messages, /v1/responses.
func (h *Handler) HandleChat(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.GetSettings()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to load settings")
		return
	}

	if settings.RequireAPIKey {
		key := auth.ExtractAPIKey(r)
		if !auth.IsValidAPIKey(key, h.store) {
			WriteError(w, http.StatusUnauthorized, "invalid or missing API key")
			return
		}
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	modelStr, _ := body["model"].(string)
	if modelStr == "" {
		WriteError(w, http.StatusBadRequest, "model field is required")
		return
	}

	modelInfo, comboModels, err := ResolveModel(modelStr, h.store)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "model resolution error: "+err.Error())
		return
	}

	endpoint := r.URL.Path
	if len(comboModels) > 0 {
		h.handleComboChat(w, r.Context(), body, comboModels, endpoint)
		return
	}

	h.handleSingleModel(w, r.Context(), body, modelInfo, endpoint)
}

// handleSingleModel executes a request against a single provider with account fallback.
func (h *Handler) handleSingleModel(w http.ResponseWriter, ctx context.Context, body map[string]interface{}, modelInfo ModelInfo, endpoint string) {
	isStream, _ := body["stream"].(bool)
	excludeID := ""
	start := time.Now()

	for {
		account, err := SelectAccount(modelInfo.Provider, excludeID, modelInfo.Model, h.store)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "account selection error: "+err.Error())
			return
		}
		if account == nil {
			WriteError(w, http.StatusServiceUnavailable, "no available accounts for provider: "+modelInfo.Provider)
			return
		}

		creds := executor.Credentials{
			APIKey:               account.APIKey,
			AccessToken:          account.AccessToken,
			RefreshToken:         account.RefreshToken,
			ProjectID:            account.ProjectID,
			CopilotToken:         account.CopilotToken,
			ProviderSpecificData: account.ProviderSpecificData,
			ConnectionID:         account.ConnectionID,
		}

		targetFormat := translator.GetTargetFormat(modelInfo.Provider)
		sourceFormat := translator.DetectSourceFormat(body)

		translatedBody, err := translator.TranslateRequest(body, sourceFormat, targetFormat)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "request translation error: "+err.Error())
			return
		}

		translatedBytes, err := json.Marshal(translatedBody)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to encode translated body")
			return
		}

		exec := executor.GetOrDefault(ResolveProviderForExecutor(modelInfo.Provider))
		result, err := exec.Execute(ctx, modelInfo.Provider, modelInfo.Model, translatedBytes, creds)
		if err != nil {
			WriteError(w, http.StatusBadGateway, "upstream error: "+err.Error())
			return
		}

		// Handle auth refresh for OAuth providers.
		if (result.StatusCode == 401 || result.StatusCode == 403) && exec.SupportsRefresh() {
			result.Body.Close()
			newCreds, refreshErr := exec.RefreshCredentials(ctx, creds)
			if refreshErr == nil && newCreds != nil {
				_ = h.store.UpdateProviderConnection(account.ConnectionID, map[string]interface{}{
					"accessToken": newCreds.AccessToken,
					"refreshToken": newCreds.RefreshToken,
				})
				result, err = exec.Execute(ctx, modelInfo.Provider, modelInfo.Model, translatedBytes, *newCreds)
				if err != nil {
					WriteError(w, http.StatusBadGateway, "upstream error after refresh: "+err.Error())
					return
				}
			}
		}

		if result.StatusCode >= 200 && result.StatusCode < 300 {
			h.writeSuccess(w, result, targetFormat, modelInfo, account, endpoint, isStream, start)
			return
		}

		// Non-success: read error body for fallback decision.
		errBody, _ := io.ReadAll(result.Body)
		result.Body.Close()
		errorText := string(errBody)

		fallback, err := MarkAccountUnavailable(account.ConnectionID, result.StatusCode, errorText, modelInfo.Provider, modelInfo.Model, h.store)
		if err != nil || !fallback {
			WriteError(w, result.StatusCode, errorText)
			return
		}

		excludeID = account.ConnectionID
	}
}

func (h *Handler) writeSuccess(w http.ResponseWriter, result *executor.ExecuteResult, targetFormat string, modelInfo ModelInfo, account *SelectedAccount, endpoint string, isStream bool, start time.Time) {
	var promptTokens, completionTokens int
	statusCode := result.StatusCode

	if result.IsStream {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(statusCode)
		st := translator.NewStreamTranslator(targetFormat, modelInfo.Model)
		promptTokens, completionTokens = StreamResponse(w, result.Body, st)
	} else {
		body, _ := io.ReadAll(result.Body)
		result.Body.Close()
		promptTokens, completionTokens = WriteJSONResponse(w, body, statusCode, targetFormat)
	}

	ClearAccountError(account.ConnectionID, h.store)
	durationMs := time.Since(start).Milliseconds()
	h.tracker.Record(modelInfo.Provider, modelInfo.Model, account.ConnectionID, endpoint, promptTokens, completionTokens, statusCode, durationMs, isStream)
}

// handleComboChat iterates combo models and stops on first success.
func (h *Handler) handleComboChat(w http.ResponseWriter, ctx context.Context, body map[string]interface{}, comboModels []string, endpoint string) {
	for _, modelStr := range comboModels {
		info, subCombo, err := ResolveModel(modelStr, h.store)
		if err != nil {
			continue
		}
		if len(subCombo) > 0 {
			// Nested combos: treat as sequential models.
			h.handleComboChat(w, ctx, body, subCombo, endpoint)
			return
		}

		rw := &responseRecorder{header: make(http.Header)}
		h.handleSingleModel(rw, ctx, body, info, endpoint)

		if rw.statusCode >= 200 && rw.statusCode < 300 {
			// Write recorded response to real writer.
			for k, vs := range rw.header {
				for _, v := range vs {
					w.Header().Set(k, v)
				}
			}
			w.WriteHeader(rw.statusCode)
			_, _ = w.Write(rw.body.Bytes())
			return
		}
	}

	WriteError(w, http.StatusServiceUnavailable, "all combo models exhausted")
}

// HandleModels handles GET /v1/models.
func (h *Handler) HandleModels(w http.ResponseWriter, r *http.Request) {
	active := true
	conns, err := h.store.GetProviderConnections(db.ConnectionFilter{IsActive: &active})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to load providers")
		return
	}

	combos, err := h.store.GetCombos()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to load combos")
		return
	}

	var models []map[string]interface{}

	seen := map[string]bool{}
	for _, c := range conns {
		id := c.Provider + "/default"
		if seen[id] {
			continue
		}
		seen[id] = true
		models = append(models, map[string]interface{}{
			"id":       id,
			"object":   "model",
			"owned_by": c.Provider,
			"created":  0,
		})
	}

	for _, combo := range combos {
		models = append(models, map[string]interface{}{
			"id":       combo.Name,
			"object":   "model",
			"owned_by": "combo",
			"created":  0,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"object": "list",
		"data":   models,
	})
}

// responseRecorder is a minimal ResponseWriter that captures response for combo retry logic.
type responseRecorder struct {
	header     http.Header
	body       bytes.Buffer
	statusCode int
}

func (rr *responseRecorder) Header() http.Header        { return rr.header }
func (rr *responseRecorder) WriteHeader(code int)       { rr.statusCode = code }
func (rr *responseRecorder) Write(b []byte) (int, error) { return rr.body.Write(b) }
