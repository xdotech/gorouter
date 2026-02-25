package server

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// DashboardProxy proxies /dashboard/* to an upstream Next.js dev server.
// Used during development when the React dashboard runs on a separate port.
func DashboardProxy(upstreamURL string) http.Handler {
	target, err := url.Parse(upstreamURL)
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "dashboard proxy misconfigured", http.StatusInternalServerError)
		})
	}
	return httputil.NewSingleHostReverseProxy(target)
}

// DashboardPlaceholder serves a minimal HTML page when no dashboard is configured.
func DashboardPlaceholder() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>gorouter</title></head>
<body style="font-family:monospace;padding:2rem">
<h1>gorouter</h1>
<p>Go port of gorouter is running.</p>
<ul>
  <li><a href="/health">Health check</a></li>
  <li><code>POST /v1/chat/completions</code> — OpenAI-compatible endpoint</li>
  <li><code>GET /v1/models</code> — List available models</li>
  <li><code>GET /api/settings</code> — Settings</li>
  <li><code>GET /api/providers</code> — Providers</li>
</ul>
<p>To use the full dashboard, run the original Next.js UI and set <code>DASHBOARD_URL=http://localhost:3000</code>.</p>
</body>
</html>`))
	})
}
