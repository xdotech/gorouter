// Package logging provides a thin slog wrapper with context-aware logging.
package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// Setup initialises the default slog logger at the given level.
func Setup(level string) {
	var l slog.Level
	switch strings.ToLower(level) {
	case "debug":
		l = slog.LevelDebug
	case "warn", "warning":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: l})
	slog.SetDefault(slog.New(handler))
}

// FromContext returns a logger enriched with any request-scoped attributes.
func FromContext(ctx context.Context) *slog.Logger {
	logger := slog.Default()
	if id, ok := ctx.Value(requestIDKey).(string); ok && id != "" {
		logger = logger.With("request_id", id)
	}
	return logger
}

// WithRequestID returns a child context carrying the given request ID.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}
