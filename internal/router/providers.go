package router

// providerAliases maps short aliases to canonical provider IDs.
var providerAliases = map[string]string{
	"cc":      "claude-code",
	"cx":      "codex",
	"gc":      "gemini-cli",
	"gh":      "github",
	"if":      "iflow",
	"qw":      "qwen",
	"kr":      "kiro",
	"glm":     "glm",
	"minimax": "minimax",
	"kimi":    "kimi",
	"kc":      "kilocode",
	"ag":      "antigravity",
	"openai":  "codex",
}

// resolveProvider returns the canonical provider ID for a given alias or ID.
func resolveProvider(alias string) string {
	if canonical, ok := providerAliases[alias]; ok {
		return canonical
	}
	return alias
}

// ResolveProviderForExecutor returns the key used to look up the executor.
// Some executors are registered under short aliases (e.g. "cc", "gc", "gh").
func ResolveProviderForExecutor(provider string) string {
	// Executors are registered under both short and canonical names in init.go,
	// so we can pass the provider directly. But for canonical names that map to
	// a short alias executor, we resolve back.
	canonicalToAlias := map[string]string{
		"claude-code": "cc",
		"gemini-cli":  "gc",
		"github":      "gh",
		"codex":       "cx",
		"antigravity": "gc", // same executor as Gemini CLI (Cloud Code API)
	}
	if alias, ok := canonicalToAlias[provider]; ok {
		return alias
	}
	return provider
}
