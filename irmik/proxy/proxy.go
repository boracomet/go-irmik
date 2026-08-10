// Package proxy provides a small reverse-proxy helper for Gin using httputil.
package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// Options configures the reverse proxy.
type Options struct {
	// Target is the upstream base URL (e.g. http://127.0.0.1:9000).
	Target string
	// StripPrefix removes this prefix from the request path before forwarding.
	StripPrefix string
	// Director optionally customizes the outgoing request after default rewrite.
	Director func(req *http.Request)
}

// Handler returns a Gin handler that reverse-proxies to Target.
func Handler(opts Options) (gin.HandlerFunc, error) {
	if opts.Target == "" {
		return nil, fmt.Errorf("proxy: Target is required")
	}
	u, err := url.Parse(opts.Target)
	if err != nil {
		return nil, fmt.Errorf("proxy: parse target: %w", err)
	}
	rp := httputil.NewSingleHostReverseProxy(u)
	defaultDirector := rp.Director
	rp.Director = func(req *http.Request) {
		defaultDirector(req)
		req.Host = u.Host
		if opts.StripPrefix != "" {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, opts.StripPrefix)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		}
		if opts.Director != nil {
			opts.Director(req)
		}
	}
	return func(c *gin.Context) {
		rp.ServeHTTP(c.Writer, c.Request)
	}, nil
}

// MustHandler is Handler that panics on configuration error.
func MustHandler(opts Options) gin.HandlerFunc {
	h, err := Handler(opts)
	if err != nil {
		panic(err)
	}
	return h
}
