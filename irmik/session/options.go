package session

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DriverFunc constructs a Store from Options.
type DriverFunc func(opts Options) (Store, error)

var (
	driversMu sync.RWMutex
	drivers   = map[string]DriverFunc{}
)

// Register adds a named session driver (e.g. "redis" via irmik/session/redisx).
// Later registrations for the same name replace the previous factory.
func Register(name string, fn DriverFunc) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || fn == nil {
		return
	}
	driversMu.Lock()
	drivers[name] = fn
	driversMu.Unlock()
}

// New creates a Store from Options (memory by default; redis via redisx).
func New(opts Options) (Store, error) {
	if opts.Store != nil {
		return opts.Store, nil
	}
	switch d := strings.ToLower(strings.TrimSpace(opts.Driver)); d {
	case "", "memory":
		return NewMemory(), nil
	default:
		driversMu.RLock()
		fn, ok := drivers[d]
		driversMu.RUnlock()
		if ok {
			return fn(opts)
		}
		if d == "redis" {
			return nil, fmt.Errorf("session: redis driver not registered; blank-import github.com/boracomet/go-irmik/irmik/session/redisx")
		}
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
