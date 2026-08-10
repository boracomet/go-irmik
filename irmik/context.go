package irmik

import (
	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/middleware"
)

// Context is a thin request wrapper around gin.Context for framework handlers.
type Context struct {
	*gin.Context
}

// FromGin wraps a gin.Context.
func FromGin(c *gin.Context) *Context {
	return &Context{Context: c}
}

// Handler is the Irmik request handler signature.
type Handler func(*Context)

// Wrap adapts an Irmik Handler to gin.HandlerFunc.
func Wrap(h Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		h(FromGin(c))
	}
}

// RequestID returns the request id set by middleware (if any).
func (c *Context) RequestID() string {
	return middleware.GetRequestID(c.Context)
}

// Param is a convenience alias for gin.Context.Param.
func (c *Context) Param(key string) string {
	return c.Context.Param(key)
}

// Query is a convenience alias for gin.Context.Query.
func (c *Context) Query(key string) string {
	return c.Context.Query(key)
}
