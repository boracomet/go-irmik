package auth

import (
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

// IssueAccessToken creates a signed JWT access token for user.
func (a *Authenticator) IssueAccessToken(user User) (string, time.Time, error) {
	if a.cfg.JWTSecret == "" {
		return "", time.Time{}, errors.New("auth: JWTSecret is required")
	}
	exp := time.Now().Add(a.cfg.AccessTTL)
	claims := Claims{
		UserID: user.ID,
		Email:  user.Email,
		Roles:  user.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			Issuer:    a.cfg.JWTIssuer,
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
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
	if !ok {
		return nil, ErrInvalidToken
	}
	return claims, nil
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
			c.Next()
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
