package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var ErrMiss = errors.New("cache miss")

// Entry holds cached HTML (or bytes) with optional expiry.
type Entry struct {
	Body        []byte
	ContentType string
	ExpiresAt   time.Time
	StaleAt     time.Time // ISR: serve stale while revalidating after this
}

func (e Entry) Expired() bool {
	if e.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(e.ExpiresAt)
}

func (e Entry) Stale() bool {
	if e.StaleAt.IsZero() {
		return e.Expired()
	}
	return time.Now().After(e.StaleAt)
}

// Store is the cache abstraction (memory / disk / redis).
type Store interface {
	Get(ctx context.Context, key string) (Entry, error)
	Set(ctx context.Context, key string, entry Entry) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
	Close() error
}

// Options configures store construction.
type Options struct {
	Driver   string
	TTL      time.Duration
	DiskDir  string
	RedisURL string
}

// DriverFunc constructs a Store from Options.
type DriverFunc func(opts Options) (Store, error)

var (
	driversMu sync.RWMutex
	drivers   = map[string]DriverFunc{}
)

// Register adds a named cache driver (e.g. "redis" via irmik/cache/redisx).
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

// New creates a Store from Options.
// Built-in: memory (default), disk. Redis requires blank-importing irmik/cache/redisx.
func New(opts Options) (Store, error) {
	switch d := strings.ToLower(strings.TrimSpace(opts.Driver)); d {
	case "", "memory":
		return NewMemory(), nil
	case "disk":
		return NewDisk(opts.DiskDir)
	default:
		driversMu.RLock()
		fn, ok := drivers[d]
		driversMu.RUnlock()
		if ok {
			return fn(opts)
		}
		if d == "redis" {
			return nil, fmt.Errorf("cache: redis driver not registered; blank-import github.com/boracomet/go-irmik/irmik/cache/redisx")
		}
		return NewMemory(), nil
	}
}
