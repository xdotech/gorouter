package gateway

import (
	"context"
	"net/http"
	"sync"
)

// codexProxy manages an on-demand HTTP listener on port 1455 for OpenAI's
// OAuth callback.  OpenAI's registered app only allows
// http://localhost:1455/auth/callback as a redirect URI, so we spin up a
// tiny proxy that receives the callback and redirects it to the main server.
type codexProxy struct {
	mu     sync.Mutex
	server *http.Server
	port   string // main server port (e.g. "14747")
}

// Start begins listening on :1455 if not already running.
func (p *codexProxy) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.server != nil {
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth/callback", func(w http.ResponseWriter, r *http.Request) {
		target := "http://localhost:" + p.port + "/api/oauth/cx/authcallback?" + r.URL.RawQuery
		http.Redirect(w, r, target, http.StatusFound)
	})

	p.server = &http.Server{Addr: ":1455", Handler: mux}
	go func() {
		if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Port may already be in use (e.g., Codex CLI running)
		}
	}()
}

// Stop shuts down the listener and releases port 1455.
func (p *codexProxy) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.server != nil {
		p.server.Shutdown(context.Background())
		p.server = nil
	}
}

// handleCodexAuthorize starts the proxy, then delegates to the OAuth handler.
func (s *Server) handleCodexAuthorize() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.cxProxy.Start()
		r.SetPathValue("provider", "cx")
		s.oh.Authorize(w, r)
	}
}

// handleCodexCallback processes the redirected callback and stops the proxy.
func (s *Server) handleCodexCallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer s.cxProxy.Stop()
		r.SetPathValue("provider", "cx")
		s.oh.Callback(w, r)
	}
}
