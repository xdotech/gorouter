package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
)

//go:embed dist/*
var dashboardFS embed.FS

// Handler serves the embedded dashboard UI from the dist folder.
func Handler() http.Handler {
	subFS, err := fs.Sub(dashboardFS, "dist")
	if err != nil {
		panic("failed to load dashboard embedded filesystem: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(subFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fileServer.ServeHTTP(w, r)
	})
}

// Proxy proxies /dashboard/* to an upstream Next.js / Vite dev server.
func Proxy(upstreamURL string) http.Handler {
	target, err := url.Parse(upstreamURL)
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "dashboard proxy misconfigured", http.StatusInternalServerError)
		})
	}
	return httputil.NewSingleHostReverseProxy(target)
}
