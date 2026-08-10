// Package audit provides a simple audit-log interface with slog and memory sinks.
// Useful for admin panels and security-sensitive actions.
package audit

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Event is one audit record.
type Event struct {
	Time     time.Time
	Actor    string // user id / email / "system"
	Action   string // e.g. "user.update"
	Resource string // e.g. "user:42"
	IP       string
	Meta     map[string]any
}

// Sink persists audit events.
type Sink interface {
	Log(ctx context.Context, e Event) error
}

// Logger writes events via slog.
type Logger struct {
	L *slog.Logger
}

// Log emits a structured slog record.
func (l Logger) Log(ctx context.Context, e Event) error {
	logger := l.L
	if logger == nil {
		logger = slog.Default()
	}
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	attrs := []any{
		"audit", true,
		"time", e.Time,
		"actor", e.Actor,
		"action", e.Action,
		"resource", e.Resource,
	}
	if e.IP != "" {
		attrs = append(attrs, "ip", e.IP)
	}
	for k, v := range e.Meta {
		attrs = append(attrs, k, v)
	}
	logger.InfoContext(ctx, "audit", attrs...)
	return nil
}

// Memory stores events in process (tests / admin debug).
type Memory struct {
	mu     sync.Mutex
	Events []Event
}

// Log appends e.
func (m *Memory) Log(ctx context.Context, e Event) error {
	_ = ctx
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	m.mu.Lock()
	m.Events = append(m.Events, e)
	m.mu.Unlock()
	return nil
}

// Snapshot returns a copy of stored events.
func (m *Memory) Snapshot() []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Event, len(m.Events))
	copy(out, m.Events)
	return out
}

// Multi fans out to multiple sinks.
type Multi []Sink

// Log writes to all sinks; returns the first error.
func (m Multi) Log(ctx context.Context, e Event) error {
	var first error
	for _, s := range m {
		if err := s.Log(ctx, e); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Record is a convenience helper that fills Time if needed.
func Record(ctx context.Context, sink Sink, e Event) error {
	if sink == nil {
		return nil
	}
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	return sink.Log(ctx, e)
}
