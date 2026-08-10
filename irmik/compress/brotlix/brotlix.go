// Package brotlix provides optional Brotli Gin middleware.
//
//	r.Use(brotlix.Brotli())
//
// Prefer gzip (irmik/compress) unless clients negotiate br.
package brotlix

import (
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
)

// Brotli returns middleware that compresses with Brotli when Accept-Encoding
// includes "br" (and not already handled).
func Brotli() gin.HandlerFunc {
	return func(c *gin.Context) {
		ae := c.GetHeader("Accept-Encoding")
		if !strings.Contains(ae, "br") {
			c.Next()
			return
		}
		if c.GetHeader("Upgrade") != "" {
			c.Next()
			return
		}
		w := brotli.NewWriterLevel(c.Writer, brotli.DefaultCompression)
		defer w.Close()
		c.Header("Content-Encoding", "br")
		c.Header("Vary", "Accept-Encoding")
		c.Writer.Header().Del("Content-Length")
		c.Writer = &brWriter{ResponseWriter: c.Writer, w: w}
		c.Next()
	}
}

type brWriter struct {
	gin.ResponseWriter
	w *brotli.Writer
}

func (b *brWriter) Write(data []byte) (int, error) {
	return b.w.Write(data)
}

func (b *brWriter) WriteString(s string) (int, error) {
	return b.w.Write([]byte(s))
}

func (b *brWriter) WriteHeader(code int) {
	b.Header().Del("Content-Length")
	b.ResponseWriter.WriteHeader(code)
}

var _ http.ResponseWriter = (*brWriter)(nil)
