package middleware

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimitConfig configures an in-memory token-bucket limiter.
type RateLimitConfig struct {
	// RPS is tokens added per second (default: 50).
	RPS float64
	// Burst is the maximum bucket size (default: max(1, 2*RPS)).
	Burst float64
	// KeyFunc extracts the client key; default uses ClientIP (trusted-proxy aware via Gin).
	KeyFunc func(*gin.Context) string
	// Skip when true, request is not limited.
	Skip func(*gin.Context) bool
}

// DefaultRateLimit is a global baseline suitable for public sites.
func DefaultRateLimit() RateLimitConfig {
	return RateLimitConfig{RPS: 50, Burst: 100}
}

// DefaultLoginRateLimit is a stricter bucket for auth endpoints.
func DefaultLoginRateLimit() RateLimitConfig {
	return RateLimitConfig{RPS: 5.0 / 60.0, Burst: 5} // ~5/min
}

// RateLimit returns Gin middleware that limits by client key (default: ClientIP).
func RateLimit(cfg RateLimitConfig) gin.HandlerFunc {
	if cfg.RPS <= 0 {
		cfg.RPS = 50
	}
	if cfg.Burst <= 0 {
		cfg.Burst = cfg.RPS * 2
		if cfg.Burst < 1 {
			cfg.Burst = 1
		}
	}
	keyFn := cfg.KeyFunc
	if keyFn == nil {
		keyFn = func(c *gin.Context) string {
			ip := c.ClientIP()
			if ip == "" {
				ip = "unknown"
			}
			return ip
		}
	}

	lim := newTokenLimiter(cfg.RPS, cfg.Burst)
	return func(c *gin.Context) {
		if cfg.Skip != nil && cfg.Skip(c) {
			c.Next()
			return
		}
		key := keyFn(c)
		if !lim.Allow(key) {
			writeTooManyRequests(c)
			return
		}
		c.Next()
	}
}

// LoginRateLimit is RateLimit with DefaultLoginRateLimit unless opts override RPS/Burst.
func LoginRateLimit(opts ...RateLimitConfig) gin.HandlerFunc {
	cfg := DefaultLoginRateLimit()
	if len(opts) > 0 {
		o := opts[0]
		if o.RPS > 0 {
			cfg.RPS = o.RPS
		}
		if o.Burst > 0 {
			cfg.Burst = o.Burst
		}
		if o.KeyFunc != nil {
			cfg.KeyFunc = o.KeyFunc
		}
		if o.Skip != nil {
			cfg.Skip = o.Skip
		}
	}
	return RateLimit(cfg)
}

type tokenLimiter struct {
	rps   float64
	burst float64
	mu    sync.Mutex
	m     map[string]*tokenBucket
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

func newTokenLimiter(rps, burst float64) *tokenLimiter {
	return &tokenLimiter{
		rps:   rps,
		burst: burst,
		m:     make(map[string]*tokenBucket),
	}
}

func (l *tokenLimiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.m[key]
	if !ok {
		l.m[key] = &tokenBucket{tokens: l.burst - 1, last: now}
		if len(l.m) > 10_000 {
			l.gcLocked(now)
		}
		return true
	}
	elapsed := now.Sub(b.last).Seconds()
	b.last = now
	b.tokens += elapsed * l.rps
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (l *tokenLimiter) gcLocked(now time.Time) {
	for k, b := range l.m {
		if now.Sub(b.last) > 10*time.Minute {
			delete(l.m, k)
		}
	}
}

// ParseTrustedProxies validates CIDR/IP strings for gin.Engine.SetTrustedProxies.
func ParseTrustedProxies(proxies []string) ([]string, error) {
	out := make([]string, 0, len(proxies))
	for _, p := range proxies {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, _, err := net.ParseCIDR(p); err == nil {
			out = append(out, p)
			continue
		}
		if ip := net.ParseIP(p); ip != nil {
			out = append(out, p)
			continue
		}
		return nil, fmt.Errorf("invalid trusted proxy %q (want IP or CIDR)", p)
	}
	return out, nil
}
