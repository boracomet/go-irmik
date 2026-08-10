package audit

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/auth"
)

// ActorFunc extracts the actor (user id / email) from the request context.
type ActorFunc func(c *gin.Context) string

// MiddlewareOptions configures the Gin audit access logger.
type MiddlewareOptions struct {
	Sink Sink
	// Actor returns the user identity; when nil, uses auth.UserFrom then "-".
	Actor ActorFunc
	// SkipPaths are exact paths or prefixes to ignore (default /health, /ready).
	SkipPaths []string
	// Action is the Event.Action value (default "http.request").
	Action string
}

// Middleware returns Gin middleware that records method/path/user/status via Sink.
func Middleware(sink Sink) gin.HandlerFunc {
	return MiddlewareWith(MiddlewareOptions{Sink: sink})
}

// MiddlewareWith returns Gin middleware from opts.
func MiddlewareWith(opts MiddlewareOptions) gin.HandlerFunc {
	sink := opts.Sink
	action := opts.Action
	if action == "" {
		action = "http.request"
	}
	actorFn := opts.Actor
	if actorFn == nil {
		actorFn = defaultActor
	}
	skips := opts.SkipPaths
	if skips == nil {
		skips = []string{"/health", "/ready"}
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path
		for _, p := range skips {
			if p == "" {
				continue
			}
			if path == p || strings.HasPrefix(path, p+"/") {
				c.Next()
				return
			}
		}
		c.Next()
		if sink == nil {
			return
		}
		_ = Record(context.Background(), sink, Event{
			Actor:    actorFn(c),
			Action:   action,
			Resource: c.Request.Method + " " + path,
			IP:       c.ClientIP(),
			Meta: map[string]any{
				"method": c.Request.Method,
				"path":   path,
				"status": c.Writer.Status(),
				"user":   actorFn(c),
			},
		})
	}
}

func defaultActor(c *gin.Context) string {
	if u, ok := auth.UserFrom(c); ok {
		if u.ID != "" {
			return u.ID
		}
		if u.Email != "" {
			return u.Email
		}
	}
	for _, key := range []string{"user_id", "userID", "uid", "email"} {
		if v, ok := c.Get(key); ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return "-"
}
