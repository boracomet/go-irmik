// Package auth provides session login helpers, JWT access tokens,
// password hashing, OAuth provider stubs, and Gin middleware.
package auth

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/session"
)

const (
	userContextKey = "irmik_auth_user"
	sessionUserKey = "auth_user_id"
)

var (
	// ErrUnauthorized is returned when authentication is required or failed.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrInvalidToken is returned when a JWT cannot be parsed or validated.
	ErrInvalidToken = errors.New("invalid token")
	// ErrInvalidCredentials is returned for failed password checks.
	ErrInvalidCredentials = errors.New("invalid credentials")
)

// User is the minimal identity stored in context / sessions.
type User struct {
	ID       string         `json:"id"`
	Email    string         `json:"email,omitempty"`
	Name     string         `json:"name,omitempty"`
	Roles    []string       `json:"roles,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Config holds auth secrets and token lifetimes.
type Config struct {
	// JWTSecret signs access tokens (required for JWT helpers).
	JWTSecret string
	// JWTIssuer is the iss claim (default: irmik).
	JWTIssuer string
	// AccessTTL is JWT lifetime (default: 15m).
	AccessTTL time.Duration
	// RefreshTTL is the rotating refresh-token lifetime (default: 7d).
	RefreshTTL time.Duration
	// SessionUserKey overrides the session key for user id.
	SessionUserKey string
}

func (c Config) withDefaults() Config {
	if c.JWTIssuer == "" {
		c.JWTIssuer = "irmik"
	}
	if c.AccessTTL <= 0 {
		c.AccessTTL = 15 * time.Minute
	}
	if c.RefreshTTL <= 0 {
		c.RefreshTTL = 7 * 24 * time.Hour
	}
	if c.SessionUserKey == "" {
		c.SessionUserKey = sessionUserKey
	}
	return c
}

// Authenticator wires session login and JWT issuance.
type Authenticator struct {
	cfg     Config
	refresh map[string]string
	revoked map[string]struct{}
	tokenMu sync.Mutex
}

// New returns an Authenticator.
func New(cfg Config) *Authenticator {
	return &Authenticator{cfg: cfg.withDefaults(), refresh: make(map[string]string), revoked: make(map[string]struct{})}
}

// Config returns a copy of the auth config.
func (a *Authenticator) Config() Config { return a.cfg }

// LoginSession regenerates the session and stores the user id.
func (a *Authenticator) LoginSession(c *gin.Context, user User) error {
	sess := session.Get(c)
	if sess == nil {
		return errors.New("auth: session middleware required")
	}
	if err := sess.Regenerate(c); err != nil {
		return err
	}
	sess.Set(a.cfg.SessionUserKey, user.ID)
	if user.Email != "" {
		sess.Set(a.cfg.SessionUserKey+"_email", user.Email)
	}
	SetUser(c, user)
	return nil
}

// LogoutSession clears auth keys and destroys the session.
func (a *Authenticator) LogoutSession(c *gin.Context) error {
	sess := session.Get(c)
	if sess == nil {
		return nil
	}
	return sess.Destroy(c)
}

// SessionUserID returns the authenticated user id from the session, if any.
func (a *Authenticator) SessionUserID(c *gin.Context) string {
	sess := session.Get(c)
	if sess == nil {
		return ""
	}
	return sess.GetString(a.cfg.SessionUserKey)
}

// InjectSessionUser loads a User from the session via lookup and stores it in context.
// lookup may return (nil, nil) for anonymous.
func (a *Authenticator) InjectSessionUser(lookup func(userID string) (*User, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := a.SessionUserID(c)
		if id == "" {
			c.Next()
			return
		}
		if lookup == nil {
			SetUser(c, User{ID: id})
			c.Next()
			return
		}
		u, err := lookup(id)
		if err != nil || u == nil {
			c.Next()
			return
		}
		SetUser(c, *u)
		c.Next()
	}
}

// RequireAuth aborts with 401 when no user is in context.
// Prefer InstallSession or InjectSessionUser / JWT middleware first.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := UserFrom(c); !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

// RequireAuthSession ensures a session user id exists (and optionally injects User{ID}).
func (a *Authenticator) RequireAuthSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := a.SessionUserID(c)
		if id == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if _, ok := UserFrom(c); !ok {
			SetUser(c, User{ID: id})
		}
		c.Next()
	}
}

// SetUser stores the authenticated user on the Gin context.
func SetUser(c *gin.Context, u User) {
	c.Set(userContextKey, u)
}

// UserFrom retrieves the authenticated user from context.
func UserFrom(c *gin.Context) (User, bool) {
	v, ok := c.Get(userContextKey)
	if !ok {
		return User{}, false
	}
	u, ok := v.(User)
	return u, ok
}

// OptionalUser returns the user or a zero value.
func OptionalUser(c *gin.Context) User {
	u, _ := UserFrom(c)
	return u
}
