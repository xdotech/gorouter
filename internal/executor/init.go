package executor

import (
	"github.com/xdotech/gorouter/internal/config"
)

// Init registers all provider executors.
// Call this once at server startup before handling any requests.
func Init(cfg *config.Config) {
	client := NewHTTPClient(cfg)

	defaultExec := newDefaultExecutor(client)
	ccExec := newClaudeCodeExecutor(client)
	gcExec := newGeminiCLIExecutor(client)
	ghExec := newGitHubExecutor(client)

	// Default executor covers all OpenAI-compatible providers and unknown ones.
	Register("default", defaultExec)
	Register("glm", defaultExec)
	Register("minimax", defaultExec)
	Register("kimi", defaultExec)
	Register("if", defaultExec)
	Register("qw", defaultExec)
	Register("openai", defaultExec)
	Register("anthropic", defaultExec)
	Register("openrouter", defaultExec)

	// Claude Code (cc)
	Register("cc", ccExec)
	Register("claude-code", ccExec)

	// Gemini CLI (gc)
	Register("gc", gcExec)
	Register("gemini-cli", gcExec)

	// GitHub Copilot (gh)
	Register("gh", ghExec)
	Register("github", ghExec)
}

// GetOrDefault returns the registered Executor for provider, or the default
// OpenAI-compatible executor if no specific one is registered.
func GetOrDefault(provider string) Executor {
	if e := Get(provider); e != nil {
		return e
	}
	return Get("default")
}
