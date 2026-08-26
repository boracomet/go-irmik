package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Claims are standard JWT claims plus user fields.
type Claims struct {
	UserID string   `json:"uid"`
	Email  string   `json:"email,omitempty"`
	Roles  []string `json:"roles,omitempty"`
	jwt.RegisteredClaims
}

// TokenPair is returned by IssueTokenPair and Refresh.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type refreshClaims struct {
	Type string `json:"typ"`
	jwt.RegisteredClaims
}

// IssueAccessToken creates a signed JWT access token for user.
func (a *Authenticator) IssueAccessToken(user User) (string, time.Time, error) {
	if a.cfg.JWTSecret == "" {
		return "", time.Time{}, errors.New("auth: JWTSecret is required")
	}
	exp := time.Now().Add(a.cfg.AccessTTL)
	jti, err := newJTI()
	if err != nil {
		return "", time.Time{}, err
	}
	claims := Claims{
		UserID: user.ID,
		Email:  user.Email,
		Roles:  user.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			Issuer:    a.cfg.JWTIssuer,
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        jti,
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(a.cfg.JWTSecret))
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

// ParseAccessToken validates and parses a JWT access token.
func (a *Authenticator) ParseAccessToken(token string) (*Claims, error) {
	if a.cfg.JWTSecret == "" {
		return nil, errors.New("auth: JWTSecret is required")
	}
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return []byte(a.cfg.JWTSecret), nil
	}, jwt.WithIssuer(a.cfg.JWTIssuer))
	if err != nil || !parsed.Valid {
		return nil, ErrInvalidToken
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || claims.ID == "" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// IssueTokenPair creates a short-lived access token and a one-time refresh token.
// The refresh jti is stored in RefreshStore. The default store is process-local
// memory with TTL/GC — not a multi-replica production backend.
func (a *Authenticator) IssueTokenPair(user User) (TokenPair, error) {
	access, exp, err := a.IssueAccessToken(user)
	if err != nil {
		return TokenPair{}, err
	}
	id, err := newJTI()
	if err != nil {
		return TokenPair{}, err
	}
	claims := refreshClaims{Type: "refresh", RegisteredClaims: jwt.RegisteredClaims{
		Subject: user.ID, Issuer: a.cfg.JWTIssuer, ID: id,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.cfg.RefreshTTL)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	refresh, err := tok.SignedString([]byte(a.cfg.JWTSecret))
	if err != nil {
		return TokenPair{}, err
	}
	if err := a.store.Put(id, user.ID, time.Now().Add(a.cfg.RefreshTTL)); err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresAt: exp}, nil
}

// Refresh rotates a refresh token. The old token is invalid after one use.
func (a *Authenticator) Refresh(token string, user User) (TokenPair, error) {
	claims, err := a.parseRefresh(token)
	if err != nil {
		return TokenPair{}, ErrInvalidToken
	}
	userID, ok := a.store.Consume(claims.ID)
	if !ok || userID != claims.Subject || a.store.Revoked(claims.Subject) || user.ID != claims.Subject {
		return TokenPair{}, ErrInvalidToken
	}
	return a.IssueTokenPair(user)
}

// RevokeUser invalidates outstanding refresh tokens for a user in RefreshStore.
func (a *Authenticator) RevokeUser(userID string) {
	a.store.RevokeUser(userID)
}

func (a *Authenticator) parseRefresh(token string) (*refreshClaims, error) {
	claims := &refreshClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return []byte(a.cfg.JWTSecret), nil
	}, jwt.WithIssuer(a.cfg.JWTIssuer))
	if err != nil || !parsed.Valid || claims.Type != "refresh" || claims.ID == "" || claims.Subject == "" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func newJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// MiddlewareJWT extracts Bearer tokens, validates them, and injects User into context.
func (a *Authenticator) MiddlewareJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		hdr := c.GetHeader("Authorization")
		if hdr == "" || !strings.HasPrefix(strings.ToLower(hdr), "bearer ") {
			c.Next()
			return
		}
		raw := strings.TrimSpace(hdr[7:])
		claims, err := a.ParseAccessToken(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		SetUser(c, User{
			ID:    claims.UserID,
			Email: claims.Email,
			Roles: claims.Roles,
		})
		c.Next()
	}
}

// RequireJWT requires a valid Bearer JWT (injects User).
func (a *Authenticator) RequireJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		hdr := c.GetHeader("Authorization")
		if hdr == "" || !strings.HasPrefix(strings.ToLower(hdr), "bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		raw := strings.TrimSpace(hdr[7:])
		claims, err := a.ParseAccessToken(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		SetUser(c, User{
			ID:    claims.UserID,
			Email: claims.Email,
			Roles: claims.Roles,
		})
		c.Next()
	}
}
