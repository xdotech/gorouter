package executor

import (
	"net/http"
	"time"

	"github.com/xuando/gorouter/internal/config"
)

// NewHTTPClient builds a shared HTTP client with proxy support and SSE-friendly settings.
// Compression is disabled so SSE streams are not buffered by the transport.
func NewHTTPClient(cfg *config.Config) *http.Client {
	transport := &http.Transport{
		Proxy:              http.ProxyFromEnvironment,
		DisableCompression: true,
	}

	// Allow explicit proxy config from the Config struct to override env vars.
	// The config already reads from HTTP_PROXY / HTTPS_PROXY / ALL_PROXY env vars,
	// so http.ProxyFromEnvironment covers the common case automatically.
	_ = cfg // reserved for future per-config overrides

	return &http.Client{
		Transport: transport,
		Timeout:   120 * time.Second,
	}
}
