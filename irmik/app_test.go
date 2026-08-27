package irmik

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/config"
	"github.com/boracomet/go-irmik/irmik/middleware"
)

func TestNewRejectsWeakJWTSecretOutsideDevelopment(t *testing.T) {
	for _, secret := range []string{"", "dev-only-change-me-jwt-secret-32b", "not-long-enough"} {
		cfg := config.Default()
		cfg.App.Env = "production"
		cfg.Auth.JWTSecret = secret
		if _, err := New(cfg); err == nil {
			t.Fatalf("New accepted production secret %q", secret)
		}
	}
}

func TestNewAllowsDevelopmentWithoutJWTSecret(t *testing.T) {
	cfg := config.Default()
	app, err := New(cfg)
	if err != nil {
		t.Fatalf("New development app: %v", err)
	}
	if app.Devtools == nil {
		t.Fatal("expected Devtools in development")
	}
}

func TestNewProductionHasNoDevtools(t *testing.T) {
	cfg := config.Default()
	cfg.App.Env = "production"
	cfg.Auth.JWTSecret = "production-jwt-secret-value-32chars"
	app, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if app.Devtools != nil {
		t.Fatal("devtools must not mount in production")
	}
}

func TestEnableSecureHeadersReplacesNotStacks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	app, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	app.Engine.GET("/hdr", func(c *gin.Context) { c.Status(http.StatusOK) })

	// Defaults from New: X-Frame-Options DENY, nosniff, no CSP.
	w := httptest.NewRecorder()
	app.Engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/hdr", nil))
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("precondition X-Frame-Options=%q", w.Header().Get("X-Frame-Options"))
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("precondition X-Content-Type-Options=%q", w.Header().Get("X-Content-Type-Options"))
	}

	app.EnableSecureHeaders(middleware.SecureHeadersConfig{
		FrameAncestors:         "'self'",
		FrameOptionsSkip:       true,
		ContentTypeOptionsSkip: true,
	})

	w = httptest.NewRecorder()
	app.Engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/hdr", nil))
	h := w.Header()
	if got := h.Get("X-Frame-Options"); got != "" {
		t.Fatalf("stale X-Frame-Options=%q after skip", got)
	}
	if got := h.Get("X-Content-Type-Options"); got != "" {
		t.Fatalf("stale X-Content-Type-Options=%q after skip", got)
	}
	if got := h.Get("Content-Security-Policy"); got != "frame-ancestors 'self'" {
		t.Fatalf("CSP=%q", got)
	}
	if h.Get("Referrer-Policy") == "" {
		t.Fatal("expected default Referrer-Policy to remain")
	}

	app.EnableSecureHeaders(middleware.SecureHeadersConfig{
		FrameAncestors: "'none'",
		FrameOptions:   "SAMEORIGIN",
	})
	w = httptest.NewRecorder()
	app.Engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/hdr", nil))
	if got := w.Header().Get("Content-Security-Policy"); got != "frame-ancestors 'none'" {
		t.Fatalf("second replace CSP=%q", got)
	}
	if got := w.Header().Get("X-Frame-Options"); got != "SAMEORIGIN" {
		t.Fatalf("second replace X-Frame-Options=%q", got)
	}
}
