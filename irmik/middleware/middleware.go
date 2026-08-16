package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/health"
)

const RequestIDHeader = "X-Request-ID"
const requestIDKey = "irmik_request_id"

// Recovery returns a Gin middleware that recovers from panics.
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, _ any) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
	})
}

// RequestID attaches a request id (from header or newly generated).
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if !validRequestID(id) {
			id = newRequestID()
		}
		c.Set(requestIDKey, id)
		c.Writer.Header().Set(RequestIDHeader, id)
		c.Next()
	}
}

func validRequestID(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			strings.ContainsRune("-_.", r)) {
			return false
		}
	}
	return true
}

// GetRequestID returns the request id stored by RequestID middleware.
func GetRequestID(c *gin.Context) string {
	if v, ok := c.Get(requestIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return c.GetHeader(RequestIDHeader)
}

// ReadyFunc reports whether the app is ready to serve traffic.
type ReadyFunc func() bool

// HealthConfig configures /health and /ready registration.
type HealthConfig struct {
	// Ready is the process-level gate (e.g. finished startup). Nil means always ready.
	Ready ReadyFunc
	// Checks are optional dependency probes. Required failures make /ready 503.
	// /health stays liveness-only (always 200 when the process is up).
	Checks *health.Registry
	// ExposeDetails returns dependency check results. Leave false in production.
	ExposeDetails bool
}

// Health registers /health and /ready on the engine.
// /health is always 200 when the process is up.
// /ready returns 200 when ready() is true (or ready is nil), else 503.
func Health(r gin.IRoutes, ready ReadyFunc) {
	HealthWith(r, HealthConfig{Ready: ready})
}

// HealthWith registers /health (liveness) and /ready (readiness + optional checks).
func HealthWith(r gin.IRoutes, cfg HealthConfig) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})
	r.GET("/ready", func(c *gin.Context) {
		ok := cfg.Ready == nil || cfg.Ready()
		body := gin.H{"ok": true}
		if cfg.Checks != nil {
			checksOK, results := cfg.Checks.Evaluate(c.Request.Context())
			if cfg.ExposeDetails {
				body["checks"] = results
			}
			if !checksOK {
				ok = false
			}
		}
		if !ok {
			body = gin.H{"ok": false}
			c.JSON(http.StatusServiceUnavailable, body)
			return
		}
		c.JSON(http.StatusOK, body)
	})
}

// ReadyFromChecks adapts a health.Registry to ReadyFunc (ignores process gate).
func ReadyFromChecks(reg *health.Registry) ReadyFunc {
	return func() bool {
		if reg == nil {
			return true
		}
		return reg.Ready(context.Background())
	}
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405.000000000")))
	}
	return hex.EncodeToString(b[:])
}
