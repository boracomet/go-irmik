// Package csrf provides CSRF token generation and Gin middleware for
// cookie/session-backed forms.
package csrf

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/session"
)

const (
	sessionKey    = "_csrf"
	contextToken  = "irmik_csrf_token"
	defaultField  = "_csrf"
	defaultHeader = "X-CSRF-Token"
)

// Options configures CSRF middleware.
type Options struct {
	// Field is the form field name (default: _csrf).
	Field string
	// Header is the header name (default: X-CSRF-Token).
	Header string
	// SafeMethods skip validation (default: GET HEAD OPTIONS TRACE).
	SafeMethods []string
	// ErrorHandler is called on failure (default: 403 JSON).
	ErrorHandler func(*gin.Context)
}

// Middleware ensures a CSRF token exists in the session and validates
// unsafe methods against form field or header.
func Middleware(opts Options) gin.HandlerFunc {
	if opts.Field == "" {
		opts.Field = defaultField
	}
	if opts.Header == "" {
		opts.Header = defaultHeader
	}
	if len(opts.SafeMethods) == 0 {
		opts.SafeMethods = []string{http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace}
	}
	safe := make(map[string]struct{}, len(opts.SafeMethods))
	for _, m := range opts.SafeMethods {
		safe[m] = struct{}{}
	}
	if opts.ErrorHandler == nil {
		opts.ErrorHandler = func(c *gin.Context) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "csrf_invalid"})
		}
	}

	return func(c *gin.Context) {
		token, ok := ensureToken(c)
		if !ok {
			opts.ErrorHandler(c)
			return
		}
		c.Set(contextToken, token)

		if _, ok := safe[c.Request.Method]; ok {
			c.Next()
			return
		}

		submitted := c.GetHeader(opts.Header)
		if submitted == "" {
			submitted = c.PostForm(opts.Field)
		}
		if !Valid(token, submitted) {
			opts.ErrorHandler(c)
			return
		}
		c.Next()
	}
}

// Ensure places a CSRF token in the session/context without validating the request.
// Useful on GET handlers that need to expose the token to clients.
func Ensure(c *gin.Context) string {
	token, ok := ensureToken(c)
	if !ok {
		return ""
	}
	c.Set(contextToken, token)
	return token
}

func ensureToken(c *gin.Context) (string, bool) {
	sess := session.Get(c)
	if sess == nil {
		return "", false
	}
	token := sess.GetString(sessionKey)
	if token == "" {
		var err error
		token, err = Generate()
		if err != nil {
			return "", false
		}
		sess.Set(sessionKey, token)
	}
	return token, true
}

// Token returns the CSRF token for the request (after Middleware).
func Token(c *gin.Context) string {
	if v, ok := c.Get(contextToken); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	if sess := session.Get(c); sess != nil {
		return sess.GetString(sessionKey)
	}
	return ""
}

// Generate creates a new random CSRF token.
func Generate() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// Valid compares expected and submitted tokens in constant time.
func Valid(expected, submitted string) bool {
	if expected == "" || submitted == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(submitted)) == 1
}
