package router

import (
	"strings"

	"github.com/xdotech/gorouter/internal/db"
)

// ModelInfo holds resolved provider and model name.
type ModelInfo struct {
	Provider string
	Model    string
}

// ParseModelString parses "provider/model" format.
// Returns empty ModelInfo if the format is invalid.
func ParseModelString(modelStr string) ModelInfo {
	idx := strings.Index(modelStr, "/")
	if idx < 1 || idx == len(modelStr)-1 {
		return ModelInfo{}
	}
	return ModelInfo{
		Provider: modelStr[:idx],
		Model:    modelStr[idx+1:],
	}
}

// ResolveModel resolves a model string to either a ModelInfo or a list of combo models.
// - If modelStr matches a combo name → returns (empty, []string{models...}, nil)
// - If modelStr is an aliased model → resolves alias, then parses
// - If modelStr is "provider/model" → returns ModelInfo
// - If modelStr is a bare model name → infers provider from known prefixes
func ResolveModel(modelStr string, store *db.Store) (ModelInfo, []string, error) {
	// Check combo first.
	combo, err := store.GetComboByName(modelStr)
	if err != nil {
		return ModelInfo{}, nil, err
	}
	if combo != nil && len(combo.Models) > 0 {
		return ModelInfo{}, combo.Models, nil
	}

	// Check model aliases in DB.
	aliases, err := store.GetModelAliases()
	if err != nil {
		return ModelInfo{}, nil, err
	}
	resolved := modelStr
	if target, ok := aliases[modelStr]; ok {
		resolved = target
	}

	// Parse "provider/model".
	info := ParseModelString(resolved)
	if info.Provider != "" {
		// Resolve provider alias.
		info.Provider = resolveProvider(info.Provider)
		return info, nil, nil
	}

	// Bare model name — infer provider from known prefixes.
	provider := inferProviderFromModel(resolved)
	return ModelInfo{Provider: provider, Model: resolved}, nil, nil
}

// inferProviderFromModel maps a bare model name to a canonical provider ID.
func inferProviderFromModel(model string) string {
	prefixMap := []struct {
		prefix   string
		provider string
	}{
		{"gemini-", "gemini-cli"},
		{"claude-", "claude-code"},
		{"gpt-", "codex"},
		{"codex", "codex"},
		{"o1", "codex"},
		{"o3", "codex"},
		{"o4", "codex"},
		{"glm-", "glm"},
		{"qwen", "qwen"},
		{"kimi", "kimi"},
		{"deepseek", "iflow"},
	}
	for _, pm := range prefixMap {
		if strings.HasPrefix(model, pm.prefix) {
			return pm.provider
		}
	}
	// Fallback: treat model as provider (legacy behavior).
	return model
}
