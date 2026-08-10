package middleware_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/health"
	"github.com/boracomet/go-irmik/irmik/middleware"
)

func TestRequestLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	r := gin.New()
	r.Use(middleware.RequestID(), middleware.RequestLog(logger))
	r.GET("/api", func(c *gin.Context) { c.Status(http.StatusCreated) })
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api", nil))
	out := buf.String()
	if !strings.Contains(out, "method=GET") || !strings.Contains(out, "path=/api") {
		t.Fatalf("log=%q", out)
	}
	if !strings.Contains(out, "status=201") || !strings.Contains(out, "request_id=") {
		t.Fatalf("log=%q", out)
	}

	buf.Reset()
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if buf.Len() != 0 {
		t.Fatalf("health should be skipped: %q", buf.String())
	}
}

func TestHealthWithChecks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reg := health.New()
	reg.Register("db", func(ctx context.Context) error {
		return errors.New("boom")
	})

	r := gin.New()
	middleware.HealthWith(r, middleware.HealthConfig{
		Ready:  func() bool { return true },
		Checks: reg,
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("liveness status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "db") {
		t.Fatalf("body=%s", w.Body.String())
	}
}
