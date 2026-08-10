package session

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when a session id is unknown or expired.
var ErrNotFound = errors.New("session not found")

// Data is the persisted session payload.
type Data struct {
	Values    map[string]any `json:"values"`
	Flash     map[string]any `json:"flash,omitempty"`
	ExpiresAt time.Time      `json:"expiresAt"`
}

// Store persists session payloads by id.
type Store interface {
	Get(ctx context.Context, id string) (Data, error)
	Save(ctx context.Context, id string, data Data) error
	Delete(ctx context.Context, id string) error
	Close() error
}

// Options configures session middleware and store construction.
type Options struct {
	// Name is the cookie name (default: irmik_session).
	Name string
	// Secret signs/encrypts cookie values when CookieSecure encoding is used.
	// For opaque id cookies, Secret is unused by the store but recommended for CSRF.
	Secret string
	// MaxAge is the session lifetime (default: 24h).
	MaxAge time.Duration
	// Path cookie path (default: /).
	Path string
	// Domain cookie domain (optional).
	Domain string
	// Secure sets the Secure cookie flag (default: true in production).
	Secure bool
	// HTTPOnly sets HttpOnly (default: true).
	HTTPOnly bool
	// SameSite cookie attribute (default: Lax).
	SameSite string // lax | strict | none
	// Driver selects the store: memory | redis (default: memory).
	Driver string
	// RedisURL for redis driver (falls back to REDIS_URL / localhost).
	RedisURL string
	// Store overrides Driver when non-nil.
	Store Store
}
