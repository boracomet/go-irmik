package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogOptions configures structured access logging.
type RequestLogOptions struct {
	// Logger defaults to slog.Default().
	Logger *slog.Logger
	// SkipPaths are not logged (default /health, /ready).
	SkipPaths []string
}

// RequestLog returns Gin middleware that logs method, path, status, latency, request-id.
func RequestLog(logger *slog.Logger) gin.HandlerFunc {
	return RequestLogWith(RequestLogOptions{Logger: logger})
}

// RequestLogWith returns Gin middleware from opts.
func RequestLogWith(opts RequestLogOptions) gin.HandlerFunc {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	skips := opts.SkipPaths
	if skips == nil {
		skips = []string{"/health", "/ready"}
	}
	skip := map[string]struct{}{}
	for _, p := range skips {
		skip[p] = struct{}{}
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if _, ok := skip[path]; ok {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		logger.Info("request",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"latency", latency.String(),
			"latency_ms", latency.Milliseconds(),
			"request_id", GetRequestID(c),
			"client_ip", c.ClientIP(),
		)
	}
}
