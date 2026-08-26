// Package sse provides Server-Sent Events helpers for Gin handlers.
package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const contentType = "text/event-stream"

// Event is one SSE message (id / event / data / retry).
type Event struct {
	ID    string
	Event string
	Data  string
	Retry time.Duration
}

// Options configures stream headers and optional keepalive.
type Options struct {
	// Heartbeat interval for comment keepalives (": ping\n\n").
	// Zero disables heartbeats.
	Heartbeat time.Duration
	// Headers are extra response headers set before the stream starts.
	Headers map[string]string
}

// Stream writes framed SSE events and cancels when the client disconnects.
type Stream struct {
	w       gin.ResponseWriter
	flusher http.Flusher
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	closed  bool
}

// New prepares the response for SSE on c and returns a Stream.
// Call Write / Event / Comment; Close cancels the stream context.
func New(c *gin.Context, opts Options) (*Stream, error) {
	w := c.Writer
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("sse: ResponseWriter does not support Flush")
	}

	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	for k, v := range opts.Headers {
		h.Set(k, v)
	}
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	// http.Server.WriteTimeout otherwise kills the stream (~30s default).
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})

	reqCtx := c.Request.Context()
	ctx, cancel := context.WithCancel(reqCtx)

	s := &Stream{
		w:       w,
		flusher: flusher,
		ctx:     ctx,
		cancel:  cancel,
	}

	go func() {
		select {
		case <-reqCtx.Done():
			s.Close()
		case <-ctx.Done():
		}
	}()

	if opts.Heartbeat > 0 {
		go s.heartbeat(opts.Heartbeat)
	}

	return s, nil
}

// Context is cancelled when the client disconnects or Close is called.
func (s *Stream) Context() context.Context { return s.ctx }

// Close cancels the stream context and stops heartbeats.
func (s *Stream) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.cancel()
}

// Done is a convenience alias for Context().Done().
func (s *Stream) Done() <-chan struct{} { return s.ctx.Done() }

// Write frames and flushes one SSE event.
func (s *Stream) Write(e Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return context.Canceled
	}
	if err := writeEvent(s.w, e); err != nil {
		s.cancel()
		return err
	}
	s.flusher.Flush()
	return nil
}

// Event writes a named event with string or JSON data.
// Non-string data is JSON-encoded.
func (s *Stream) Event(name string, data any) error {
	payload, err := encodeData(data)
	if err != nil {
		return err
	}
	return s.Write(Event{Event: name, Data: payload})
}

// Data writes a default (unnamed) message event.
func (s *Stream) Data(data any) error {
	payload, err := encodeData(data)
	if err != nil {
		return err
	}
	return s.Write(Event{Data: payload})
}

// Comment writes an SSE comment line (useful for keepalives).
func (s *Stream) Comment(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return context.Canceled
	}
	if text == "" {
		text = "ping"
	}
	if _, err := fmt.Fprintf(s.w, ": %s\n\n", text); err != nil {
		s.cancel()
		return err
	}
	s.flusher.Flush()
	return nil
}

func (s *Stream) heartbeat(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-t.C:
			if err := s.Comment("keepalive"); err != nil {
				return
			}
		}
	}
}

// Handler returns a Gin handler that opens an SSE stream and runs fn.
// The stream is closed when fn returns or the client disconnects.
func Handler(opts Options, fn func(*Stream) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, err := New(c, opts)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer s.Close()
		_ = fn(s)
	}
}

// Format encodes an Event into the wire format without flushing.
func Format(e Event) string {
	var b strings.Builder
	_ = writeEvent(&b, e)
	return b.String()
}

func encodeData(data any) (string, error) {
	switch v := data.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

func writeEvent(w io.Writer, e Event) error {
	if e.ID != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", sanitizeField(e.ID)); err != nil {
			return err
		}
	}
	if e.Event != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", sanitizeField(e.Event)); err != nil {
			return err
		}
	}
	if e.Retry > 0 {
		ms := e.Retry.Milliseconds()
		if _, err := fmt.Fprintf(w, "retry: %s\n", strconv.FormatInt(ms, 10)); err != nil {
			return err
		}
	}
	data := e.Data
	if data == "" && e.Event == "" && e.ID == "" && e.Retry == 0 {
		data = ""
	}
	for _, line := range strings.Split(data, "\n") {
		if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func sanitizeField(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\n", ""), "\r", "")
}
