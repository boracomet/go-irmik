package devtools

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const maxInjectBytes = 4 << 20

type bodyWriter struct {
	gin.ResponseWriter
	buf         bytes.Buffer
	status      int
	wroteHeader bool
	skipped     bool
}

func (w *bodyWriter) WriteHeader(code int) {
	w.status = code
	w.wroteHeader = true
}

func (w *bodyWriter) Write(data []byte) (int, error) {
	if w.skipped {
		w.flushHeader()
		return w.ResponseWriter.Write(data)
	}
	if w.buf.Len()+len(data) > maxInjectBytes {
		w.skipped = true
		w.flushHeader()
		if w.buf.Len() > 0 {
			if _, err := w.ResponseWriter.Write(w.buf.Bytes()); err != nil {
				return 0, err
			}
			w.buf.Reset()
		}
		return w.ResponseWriter.Write(data)
	}
	return w.buf.Write(data)
}

func (w *bodyWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *bodyWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *bodyWriter) Size() int {
	if w.skipped {
		return w.ResponseWriter.Size()
	}
	return w.buf.Len()
}

func (w *bodyWriter) Written() bool {
	return w.wroteHeader || w.buf.Len() > 0 || w.skipped
}

func (w *bodyWriter) flushHeader() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.ResponseWriter.WriteHeader(w.status)
	w.wroteHeader = true
}

// Inject wraps HTML responses and inserts the overlay script.
func (d *Dev) Inject() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodHead {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/_irmik/dev") {
			c.Next()
			return
		}
		bw := &bodyWriter{ResponseWriter: c.Writer, status: http.StatusOK}
		c.Writer = bw
		c.Next()

		if bw.skipped {
			return
		}
		status := bw.Status()
		ct := bw.Header().Get("Content-Type")
		body := bw.buf.Bytes()

		if strings.Contains(ct, "text/event-stream") || strings.Contains(ct, "application/json") {
			bw.flushHeader()
			_, _ = bw.ResponseWriter.Write(body)
			return
		}

		htmlish := strings.Contains(ct, "text/html") ||
			(status >= 400 && strings.Contains(ct, "text/plain") && wantsHTML(c))
		if !htmlish || bytes.Contains(body, []byte("/_irmik/dev/overlay.js")) {
			bw.flushHeader()
			_, _ = bw.ResponseWriter.Write(body)
			return
		}

		if strings.Contains(ct, "text/plain") {
			body = []byte(wrapPlainError(string(body)))
			bw.Header().Set("Content-Type", "text/html; charset=utf-8")
		}

		out := injectHTML(body)
		bw.Header().Del("Content-Length")
		bw.ResponseWriter.WriteHeader(status)
		_, _ = bw.ResponseWriter.Write(out)
	}
}

func wantsHTML(c *gin.Context) bool {
	return strings.Contains(c.GetHeader("Accept"), "text/html")
}

func injectHTML(body []byte) []byte {
	tag := []byte(snippet())
	lower := bytes.ToLower(body)
	if i := bytes.LastIndex(lower, []byte("</body>")); i >= 0 {
		out := make([]byte, 0, len(body)+len(tag))
		out = append(out, body[:i]...)
		out = append(out, tag...)
		out = append(out, body[i:]...)
		return out
	}
	out := make([]byte, 0, len(body)+len(tag))
	out = append(out, body...)
	out = append(out, tag...)
	return out
}
