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
	if info.Provider == "" {
		// Treat as provider-only (no model specified); caller handles.
		return ModelInfo{Provider: resolved, Model: ""}, nil, nil
	}

	// Resolve provider alias.
	info.Provider = resolveProvider(info.Provider)
	return info, nil, nil
}
