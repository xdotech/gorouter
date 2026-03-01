package server

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/xdotech/gorouter/internal/config"
)

type Server struct {
	cfg    *config.Config
	router chi.Router
}

func New(cfg *config.Config) *Server {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	return &Server{cfg: cfg, router: r}
}

func (s *Server) Mount(fn func(r chi.Router)) {
	fn(s.router)
}

func (s *Server) Router() chi.Router {
	return s.router
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%s", s.cfg.Host, s.cfg.Port)
	fmt.Printf("gorouter (Go) started — http://%s\n", addr)
	return http.ListenAndServe(addr, s.router)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, x-api-key, anthropic-version, anthropic-beta")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
