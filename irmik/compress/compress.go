// Package compress provides Gin middleware for response compression.
// Gzip uses the standard library. For Brotli, import irmik/compress/brotlix.
package compress

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var gzipPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return w
	},
}

// Gzip returns middleware that gzip-compresses responses when the client
// accepts gzip and the response is compressible.
func Gzip() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}
		// Skip already-encoded or websocket upgrades.
		if c.GetHeader("Upgrade") != "" {
			c.Next()
			return
		}

		gw := gzipPool.Get().(*gzip.Writer)
		gw.Reset(c.Writer)
		defer func() {
			_ = gw.Close()
			gzipPool.Put(gw)
		}()

		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")
		c.Writer.Header().Del("Content-Length")
		c.Writer = &gzipWriter{ResponseWriter: c.Writer, w: gw}
		c.Next()
	}
}

type gzipWriter struct {
	gin.ResponseWriter
	w *gzip.Writer
}

func (g *gzipWriter) Write(data []byte) (int, error) {
	return g.w.Write(data)
}

func (g *gzipWriter) WriteString(s string) (int, error) {
	return g.w.Write([]byte(s))
}

// WriteHeader ensures Content-Length is cleared for compressed bodies.
func (g *gzipWriter) WriteHeader(code int) {
	g.Header().Del("Content-Length")
	g.ResponseWriter.WriteHeader(code)
}

// Ensure gzipWriter implements http.ResponseWriter.
var _ http.ResponseWriter = (*gzipWriter)(nil)
