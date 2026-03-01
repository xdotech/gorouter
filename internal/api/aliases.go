package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/xdotech/gorouter/internal/db"
)

// AliasesHandler manages model aliases.
type AliasesHandler struct {
	store *db.Store
}

func NewAliasesHandler(store *db.Store) *AliasesHandler {
	return &AliasesHandler{store: store}
}

// List GET /api/models/alias
func (h *AliasesHandler) List(w http.ResponseWriter, r *http.Request) {
	aliases, err := h.store.GetModelAliases()
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to list aliases")
		return
	}
	JSON(w, http.StatusOK, aliases)
}

// Set POST /api/models/alias
// Body: {"alias": "foo", "target": "cc/claude-opus-4-6"}
func (h *AliasesHandler) Set(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Alias  string `json:"alias"`
		Target string `json:"target"`
	}
	if err := DecodeBody(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Alias == "" || body.Target == "" {
		Error(w, http.StatusBadRequest, "alias and target required")
		return
	}
	if err := h.store.SetModelAlias(body.Alias, body.Target); err != nil {
		Error(w, http.StatusInternalServerError, "failed to save alias")
		return
	}
	JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Delete DELETE /api/models/alias/{alias}
func (h *AliasesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	alias := chi.URLParam(r, "alias")
	if err := h.store.DeleteModelAlias(alias); err != nil {
		Error(w, http.StatusInternalServerError, "failed to delete alias")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
