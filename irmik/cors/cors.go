// Package cors provides a lean CORS middleware for Gin (no heavy dependency).
// Opt-in: mount with r.Use(cors.Middleware(opts)) or app.Engine.Use(...).
package cors

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Options configures Cross-Origin Resource Sharing.
type Options struct {
	// AllowOrigins lists permitted Origin values. Use "*" for any (not with credentials).
	AllowOrigins []string
	// AllowMethods default: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS.
	AllowMethods []string
	// AllowHeaders default: Accept, Authorization, Content-Type, X-Request-ID.
	AllowHeaders []string
	// ExposeHeaders are visible to the browser (optional).
	ExposeHeaders []string
	// AllowCredentials sets Access-Control-Allow-Credentials.
	AllowCredentials bool
	// MaxAge is preflight cache seconds (0 omits the header).
	MaxAge int
}

// Default returns common methods and headers but no permitted origins. Add explicit
// AllowOrigins before mounting it in an application.
func Default() Options {
	return Options{
		AllowMethods: []string{
			http.MethodGet, http.MethodPost, http.MethodPut,
			http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions,
		},
		AllowHeaders: []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		MaxAge:       600,
	}
}

// DefaultDev returns permissive local-development options. It must not be used
// with credentials; Middleware will not reflect origins for that combination.
func DefaultDev() Options {
	opts := Default()
	opts.AllowOrigins = []string{"*"}
	return opts
}

// Middleware returns Gin CORS middleware from opts.
func Middleware(opts Options) gin.HandlerFunc {
	if len(opts.AllowMethods) == 0 {
		opts.AllowMethods = Default().AllowMethods
	}
	if len(opts.AllowHeaders) == 0 {
		opts.AllowHeaders = Default().AllowHeaders
	}
	methods := strings.Join(opts.AllowMethods, ", ")
	headers := strings.Join(opts.AllowHeaders, ", ")
	expose := strings.Join(opts.ExposeHeaders, ", ")
	origins := make(map[string]struct{}, len(opts.AllowOrigins))
	allowAll := false
	for _, o := range opts.AllowOrigins {
		if o == "*" {
			allowAll = true
			continue
		}
		origins[o] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			allowed := ""
			if allowAll && !opts.AllowCredentials {
				allowed = "*"
			} else if _, ok := origins[origin]; ok {
				allowed = origin
			}
			if allowed != "" {
				c.Header("Access-Control-Allow-Origin", allowed)
				if opts.AllowCredentials {
					c.Header("Access-Control-Allow-Credentials", "true")
				}
				if expose != "" {
					c.Header("Access-Control-Expose-Headers", expose)
				}
				c.Header("Vary", "Origin")
			}
		}

		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Methods", methods)
			c.Header("Access-Control-Allow-Headers", headers)
			if opts.MaxAge > 0 {
				c.Header("Access-Control-Max-Age", strconv.Itoa(opts.MaxAge))
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
