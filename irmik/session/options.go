package session

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// New creates a Store from Options (memory or redis).
func New(opts Options) (Store, error) {
	if opts.Store != nil {
		return opts.Store, nil
	}
	switch strings.ToLower(strings.TrimSpace(opts.Driver)) {
	case "", "memory":
		return NewMemory(), nil
	case "redis":
		return NewRedis(opts.RedisURL)
	default:
		return nil, fmt.Errorf("session: unknown driver %q", opts.Driver)
	}
}

func normalizeOptions(opts Options) Options {
	if opts.Name == "" {
		opts.Name = "irmik_session"
	}
	if opts.MaxAge <= 0 {
		opts.MaxAge = 24 * time.Hour
	}
	if opts.Path == "" {
		opts.Path = "/"
	}
	if opts.SameSite == "" {
		opts.SameSite = "lax"
	}
	return opts
}

func sameSiteMode(v string) http.SameSite {
	switch strings.ToLower(v) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}
