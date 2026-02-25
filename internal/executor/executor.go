package executor

import (
	"context"
	"io"
	"net/http"
	"sync"
)

// Credentials holds provider authentication data.
type Credentials struct {
	APIKey               string
	AccessToken          string
	RefreshToken         string
	ProjectID            string
	CopilotToken         string
	ProviderSpecificData map[string]interface{}
	ConnectionID         string
}

// ExecuteResult carries the upstream response back to the caller.
// The caller is responsible for closing Body.
type ExecuteResult struct {
	Body       io.ReadCloser
	StatusCode int
	Headers    http.Header
	IsStream   bool
}

// Executor sends a translated request to an upstream provider.
type Executor interface {
	Execute(ctx context.Context, provider, model string, bodyBytes []byte, creds Credentials) (*ExecuteResult, error)
	RefreshCredentials(ctx context.Context, creds Credentials) (*Credentials, error)
	SupportsRefresh() bool
}

var (
	mu       sync.RWMutex
	registry = map[string]Executor{}
)

// Register associates a provider name with an Executor implementation.
func Register(provider string, e Executor) {
	mu.Lock()
	defer mu.Unlock()
	registry[provider] = e
}

// Get returns the Executor registered for provider, or nil if none.
func Get(provider string) Executor {
	mu.RLock()
	defer mu.RUnlock()
	return registry[provider]
}
