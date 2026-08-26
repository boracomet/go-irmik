// Package queue provides a small job-queue interface and an in-memory implementation
// with a worker Run loop.
//
// Redis/asynq is opt-in via irmik/queue/asynqx so core binaries stay free of
// asynq and Redis client deps:
//
//	import _ "github.com/boracomet/go-irmik/irmik/queue/asynqx" // registers "asynq"
//	q, err := queue.New(queue.Options{Driver: "asynq", RedisURL: "redis://localhost:6379/0"})
//
// Or open explicitly without blank-import:
//
//	q, err := asynqx.Open(asynqx.Options{RedisURL: "redis://localhost:6379/0"})
package queue

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ErrClosed is returned when enqueueing on a closed queue.
var ErrClosed = errors.New("queue: closed")

// Job is a unit of work.
type Job struct {
	Name    string
	Payload []byte
	// At, when non-zero, delays availability until that time (best-effort for Memory).
	At time.Time
}

// Handler processes a job. Returning an error may trigger retry depending on backend.
type Handler func(ctx context.Context, job Job) error

// Queue enqueues jobs and runs workers.
type Queue interface {
	Enqueue(ctx context.Context, job Job) error
	// Run blocks, invoking h for each job until ctx is cancelled or Close.
	Run(ctx context.Context, h Handler) error
	Close() error
}

// Options configures queue.New.
type Options struct {
	// Driver selects the backend: "memory" (default) or a registered name such as "asynq".
	Driver string
	// Buffer is the in-memory channel size (memory driver only; default 64).
	Buffer int
	// RedisURL is passed to registered Redis-backed drivers (e.g. asynq).
	RedisURL string
	// Concurrency is worker concurrency for backends that support it (default backend-specific).
	Concurrency int
	// QueueName is the named queue for backends that support multiple queues (asynq default "default").
	QueueName string
}

// DriverFunc constructs a Queue from Options.
type DriverFunc func(opts Options) (Queue, error)

var (
	driversMu sync.RWMutex
	drivers   = map[string]DriverFunc{}
)

// Register adds a named queue driver (e.g. "asynq" via irmik/queue/asynqx).
// Later registrations for the same name replace the previous factory.
func Register(name string, fn DriverFunc) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || fn == nil {
		return
	}
	driversMu.Lock()
	drivers[name] = fn
	driversMu.Unlock()
}

// New creates a Queue from Options.
// Built-in: memory (default). Asynq requires blank-importing irmik/queue/asynqx
// or calling asynqx.Open directly.
func New(opts Options) (Queue, error) {
	switch d := strings.ToLower(strings.TrimSpace(opts.Driver)); d {
	case "", "memory":
		return NewMemory(opts.Buffer), nil
	default:
		driversMu.RLock()
		fn, ok := drivers[d]
		driversMu.RUnlock()
		if ok {
			return fn(opts)
		}
		if d == "asynq" || d == "redis" {
			return nil, fmt.Errorf("queue: %s driver not registered; blank-import github.com/boracomet/go-irmik/irmik/queue/asynqx", d)
		}
		return nil, fmt.Errorf("queue: unknown driver %q", d)
	}
}

// Memory is a buffered in-process queue.
type Memory struct {
	ch     chan Job
	quit   chan struct{}
	mu     sync.Mutex
	closed bool
	wg     sync.WaitGroup
}

// NewMemory creates an in-memory queue with the given buffer size (default 64).
func NewMemory(buffer int) *Memory {
	if buffer <= 0 {
		buffer = 64
	}
	return &Memory{ch: make(chan Job, buffer), quit: make(chan struct{})}
}

// Enqueue adds a job. Delayed jobs (Job.At in the future) sleep in a goroutine
// before entering the channel.
func (m *Memory) Enqueue(ctx context.Context, job Job) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return ErrClosed
	}
	if !job.At.IsZero() && time.Until(job.At) > 0 {
		delay := time.Until(job.At)
		m.wg.Add(1)
		m.mu.Unlock()
		go m.enqueueDelayed(ctx, job, delay)
		return nil
	}
	m.mu.Unlock()
	return m.send(ctx, job)
}

func (m *Memory) enqueueDelayed(ctx context.Context, job Job, delay time.Duration) {
	defer m.wg.Done()
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-m.quit:
		return
	case <-timer.C:
	}
	_ = m.send(ctx, job)
}

func (m *Memory) send(ctx context.Context, job Job) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.quit:
		return ErrClosed
	case m.ch <- job:
		return nil
	}
}

// Run consumes jobs until ctx is done or Close.
func (m *Memory) Run(ctx context.Context, h Handler) error {
	if h == nil {
		return errors.New("queue: nil handler")
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-m.quit:
			return nil
		case job, ok := <-m.ch:
			if !ok {
				return nil
			}
			if err := h(ctx, job); err != nil {
				// In-memory: surface error to caller via return only if ctx cancelled mid-run.
				// Keep processing; handlers should log/retry themselves for now.
				_ = err
			}
		}
	}
}

// Close stops accepting jobs and waits for delayed enqueue goroutines.
// The job channel is left open so a racing delayed send cannot panic.
func (m *Memory) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	close(m.quit)
	m.mu.Unlock()
	m.wg.Wait()
	return nil
}
