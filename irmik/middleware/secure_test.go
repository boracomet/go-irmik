package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/middleware"
)

func TestSecureHeadersDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.SecureHeaders(middleware.DefaultSecureHeaders()))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	h := w.Header()
	if h.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("X-Content-Type-Options=%q", h.Get("X-Content-Type-Options"))
	}
	if h.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("X-Frame-Options=%q", h.Get("X-Frame-Options"))
	}
	if h.Get("Referrer-Policy") == "" {
		t.Fatal("missing Referrer-Policy")
	}
	if h.Get("Permissions-Policy") == "" {
		t.Fatal("missing Permissions-Policy")
	}
	if h.Get("Strict-Transport-Security") != "" {
		t.Fatal("HSTS should be off by default")
	}
}

func TestSecureHeadersSkip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.SecureHeaders(middleware.SecureHeadersConfig{
		FrameAncestors:         "'self'",
		FrameOptionsSkip:       true,
		ContentTypeOptionsSkip: true,
	}))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	h := w.Header()
	if h.Get("X-Frame-Options") != "" {
		t.Fatalf("X-Frame-Options=%q", h.Get("X-Frame-Options"))
	}
	if h.Get("X-Content-Type-Options") != "" {
		t.Fatalf("X-Content-Type-Options=%q", h.Get("X-Content-Type-Options"))
	}
	if got := h.Get("Content-Security-Policy"); got != "frame-ancestors 'self'" {
		t.Fatalf("CSP=%q", got)
	}
}

func TestSecureHeadersHSTS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.SecureHeaders(middleware.SecureHeadersWithHSTS(3600)))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	got := w.Header().Get("Strict-Transport-Security")
	if got == "" || got[:8] != "max-age=" {
		t.Fatalf("HSTS=%q", got)
	}
}

func TestRateLimitBlocksBurst(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RateLimit(middleware.RateLimitConfig{RPS: 1, Burst: 2}))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	codes := make([]int, 0, 4)
	for i := 0; i < 4; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.10:1234"
		r.ServeHTTP(w, req)
		codes = append(codes, w.Code)
	}
	ok, limited := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusOK:
			ok++
		case http.StatusTooManyRequests:
			limited++
		}
	}
	if ok != 2 || limited != 2 {
		t.Fatalf("codes=%v want 2 OK + 2 limited", codes)
	}
}

func TestLoginRateLimitHelper(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/login", middleware.LoginRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	for i := 0; i < 6; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "198.51.100.1:9"
		r.ServeHTTP(w, req)
		if i < 5 && w.Code != http.StatusOK {
			t.Fatalf("request %d: status=%d", i, w.Code)
		}
		if i == 5 && w.Code != http.StatusTooManyRequests {
			t.Fatalf("request %d: want 429, got %d", i, w.Code)
		}
	}
}
