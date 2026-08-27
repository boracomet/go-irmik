// Package observe provides structured slog helpers for Irmik apps.
// For OpenTelemetry metrics/traces, go get the nested module
// github.com/boracomet/go-irmik/irmik/observe/otelx.
package observe

import (
	"context"
	"log/slog"
	"os"
)

// Options configures the default logger.
type Options struct {
	// Level is slog level (default Info).
	Level slog.Level
	// JSON selects JSONHandler instead of TextHandler.
	JSON bool
	// Service is attached as attr "service" when non-empty.
	Service string
}

// NewLogger builds a slog.Logger from Options.
func NewLogger(opts Options) *slog.Logger {
	var h slog.Handler
	level := &slog.LevelVar{}
	level.Set(opts.Level)
	ho := &slog.HandlerOptions{Level: level}
	if opts.JSON {
		h = slog.NewJSONHandler(os.Stdout, ho)
	} else {
		h = slog.NewTextHandler(os.Stdout, ho)
	}
	logger := slog.New(h)
	if opts.Service != "" {
		logger = logger.With("service", opts.Service)
	}
	return logger
}

// SetDefault installs logger as slog default.
func SetDefault(logger *slog.Logger) {
	slog.SetDefault(logger)
}

type ctxKey struct{}

// WithLogger stores logger in ctx.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, logger)
}

// FromContext returns the logger from ctx, or slog.Default().
func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

// RequestAttrs returns common HTTP attrs for logging.
func RequestAttrs(method, path, requestID string) []any {
	attrs := []any{"method", method, "path", path}
	if requestID != "" {
		attrs = append(attrs, "request_id", requestID)
	}
	return attrs
}
