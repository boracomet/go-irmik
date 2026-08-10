package cache

import (
	"context"
	"errors"
	"time"
)

var ErrMiss = errors.New("cache miss")

// Entry holds cached HTML (or bytes) with optional expiry.
type Entry struct {
	Body      []byte
	ContentType string
	ExpiresAt time.Time
	StaleAt   time.Time // ISR: serve stale while revalidating after this
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

// New creates a Store from Options.
func New(opts Options) (Store, error) {
	switch opts.Driver {
	case "", "memory":
		return NewMemory(), nil
	case "disk":
		return NewDisk(opts.DiskDir)
	case "redis":
		return NewRedis(opts.RedisURL)
	default:
		return NewMemory(), nil
	}
}
