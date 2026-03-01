package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/xdotech/gorouter/internal/auth"
	"github.com/xdotech/gorouter/internal/config"
	"github.com/xdotech/gorouter/internal/logging"
)

type contextKey string

const requestIDKey contextKey = "request_id"

func getRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// requestIDMiddleware assigns a unique ID to each request.
func requestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := r.Header.Get("X-Request-Id")
			if reqID == "" {
				b := make([]byte, 8)
				_, _ = rand.Read(b)
				reqID = hex.EncodeToString(b)
			}
			ctx := context.WithValue(r.Context(), requestIDKey, reqID)
			w.Header().Set("X-Request-Id", reqID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	status       int
	bytesWritten int
	wroteHeader  bool
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, status: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.status = code
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.wroteHeader = true
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// loggerMiddleware logs requests using slog.
func loggerMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := logging.WithRequestID(r.Context(), getRequestID(r.Context()))
			logger := logging.FromContext(ctx)

			ww := newResponseWriter(w)
			start := time.Now()

			defer func() {
				logger.Info("request completed",
					"method", r.Method,
					"path", r.URL.Path,
					"status", ww.status,
					"duration", time.Since(start).String(),
					"bytes", ww.bytesWritten,
				)
			}()

			next.ServeHTTP(ww, r.WithContext(ctx))
		})
	}
}

// recovererMiddleware catches panics and returns 500.
func recovererMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					logger := logging.FromContext(r.Context())
					logger.Error("panic recovered", "panic", rvr, "stack", string(debug.Stack()))
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// corsMiddleware sets CORS headers.
func corsMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-Api-Key, X-CSRF-Token")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// loginRequired checks for a valid JWT cookie.
// If requireLogin setting is false, it skips the check.
func (s *Server) loginRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		settings, err := s.stores.Settings.Get()
		if err != nil {
			renderError(w, http.StatusInternalServerError, "failed to load settings")
			return
		}
		if !settings.RequireLogin {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie("auth_token")
		if err != nil || cookie.Value == "" {
			renderError(w, http.StatusUnauthorized, "login required")
			return
		}

		valid, err := auth.ValidateToken(cookie.Value, s.cfg.JWTSecret)
		if err != nil || !valid {
			renderError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// apiKeyRequired validates the API key if required by settings.
func (s *Server) apiKeyRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.cfg.RequireAPIKey {
			next.ServeHTTP(w, r)
			return
		}

		key := extractAPIKey(r)
		if key == "" {
			renderError(w, http.StatusUnauthorized, "API key required")
			return
		}

		if !s.stores.APIKeys.ValidateKey(key) {
			renderError(w, http.StatusUnauthorized, "invalid API key")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func extractAPIKey(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if strings.HasPrefix(h, "Bearer ") {
			return strings.TrimPrefix(h, "Bearer ")
		}
	}
	return r.Header.Get("x-api-key")
}
