package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// SecureHeadersConfig configures baseline browser security headers.
// Zero values use sensible defaults. Set Skip to disable a header.
type SecureHeadersConfig struct {
	// ContentTypeOptions default: nosniff. Empty with ContentTypeOptionsSkip skips.
	ContentTypeOptions     string
	ContentTypeOptionsSkip bool

	// FrameOptions default: DENY. Prefer FrameAncestors for CSP-based framing control.
	FrameOptions     string
	FrameOptionsSkip bool

	// FrameAncestors when set, emits Content-Security-Policy: frame-ancestors <value>
	// and skips X-Frame-Options unless FrameOptions is also set explicitly.
	FrameAncestors string

	// ReferrerPolicy default: strict-origin-when-cross-origin.
	ReferrerPolicy     string
	ReferrerPolicySkip bool

	// PermissionsPolicy baseline; empty uses a conservative default.
	PermissionsPolicy     string
	PermissionsPolicySkip bool

	// HSTSMaxAge seconds for Strict-Transport-Security. 0 disables HSTS.
	// Use EnableHSTS or production SecureDefaults to turn on.
	HSTSMaxAge            int
	HSTSIncludeSubdomains bool
	HSTSPreload           bool
}

// DefaultSecureHeaders returns cheap baseline headers without HSTS.
func DefaultSecureHeaders() SecureHeadersConfig {
	return SecureHeadersConfig{
		ContentTypeOptions: "nosniff",
		FrameOptions:       "DENY",
		ReferrerPolicy:     "strict-origin-when-cross-origin",
		PermissionsPolicy:  "camera=(), microphone=(), geolocation=()",
	}
}

// SecureHeaders sets baseline security response headers.
func SecureHeaders(cfg SecureHeadersConfig) gin.HandlerFunc {
	return SecureHeadersFunc(func() SecureHeadersConfig { return cfg })
}

// SecureHeadersFunc re-reads config on each request so a live swap
// (App.EnableSecureHeaders) replaces headers instead of stacking middleware.
func SecureHeadersFunc(load func() SecureHeadersConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		writeSecureHeaders(c.Writer.Header(), load())
		c.Next()
	}
}

func writeSecureHeaders(h http.Header, cfg SecureHeadersConfig) {
	cfg = normalizeSecureHeaders(cfg)
	if !cfg.ContentTypeOptionsSkip && cfg.ContentTypeOptions != "" {
		h.Set("X-Content-Type-Options", cfg.ContentTypeOptions)
	} else {
		h.Del("X-Content-Type-Options")
	}
	if cfg.FrameAncestors != "" {
		h.Set("Content-Security-Policy", "frame-ancestors "+cfg.FrameAncestors)
		if !cfg.FrameOptionsSkip && cfg.FrameOptions != "" {
			h.Set("X-Frame-Options", cfg.FrameOptions)
		} else {
			h.Del("X-Frame-Options")
		}
	} else if !cfg.FrameOptionsSkip && cfg.FrameOptions != "" {
		h.Set("X-Frame-Options", cfg.FrameOptions)
		h.Del("Content-Security-Policy")
	} else {
		h.Del("X-Frame-Options")
		h.Del("Content-Security-Policy")
	}
	if !cfg.ReferrerPolicySkip && cfg.ReferrerPolicy != "" {
		h.Set("Referrer-Policy", cfg.ReferrerPolicy)
	} else {
		h.Del("Referrer-Policy")
	}
	if !cfg.PermissionsPolicySkip && cfg.PermissionsPolicy != "" {
		h.Set("Permissions-Policy", cfg.PermissionsPolicy)
	} else {
		h.Del("Permissions-Policy")
	}
	if cfg.HSTSMaxAge > 0 {
		parts := []string{"max-age=" + strconv.Itoa(cfg.HSTSMaxAge)}
		if cfg.HSTSIncludeSubdomains {
			parts = append(parts, "includeSubDomains")
		}
		if cfg.HSTSPreload {
			parts = append(parts, "preload")
		}
		h.Set("Strict-Transport-Security", strings.Join(parts, "; "))
	} else {
		h.Del("Strict-Transport-Security")
	}
}

func normalizeSecureHeaders(cfg SecureHeadersConfig) SecureHeadersConfig {
	def := DefaultSecureHeaders()
	if !cfg.ContentTypeOptionsSkip && cfg.ContentTypeOptions == "" {
		cfg.ContentTypeOptions = def.ContentTypeOptions
	}
	if cfg.FrameAncestors == "" && !cfg.FrameOptionsSkip && cfg.FrameOptions == "" {
		cfg.FrameOptions = def.FrameOptions
	}
	if !cfg.ReferrerPolicySkip && cfg.ReferrerPolicy == "" {
		cfg.ReferrerPolicy = def.ReferrerPolicy
	}
	if !cfg.PermissionsPolicySkip && cfg.PermissionsPolicy == "" {
		cfg.PermissionsPolicy = def.PermissionsPolicy
	}
	return cfg
}

// SecureHeadersWithHSTS is DefaultSecureHeaders plus HSTS (typical production HTTPS).
func SecureHeadersWithHSTS(maxAgeSeconds int) SecureHeadersConfig {
	cfg := DefaultSecureHeaders()
	if maxAgeSeconds <= 0 {
		maxAgeSeconds = 31536000 // 1 year
	}
	cfg.HSTSMaxAge = maxAgeSeconds
	cfg.HSTSIncludeSubdomains = true
	return cfg
}

// NoStore is a small helper used by rate-limit error responses.
func writeTooManyRequests(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"error": "rate limit exceeded",
	})
}
