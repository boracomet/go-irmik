// Package queue provides a small job-queue interface and an in-memory implementation
// with a worker Run loop. Redis/asynq backends can implement Queue later without
// changing callers.
package queue

import (
	"context"
	"errors"
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

// Memory is a buffered in-process queue.
type Memory struct {
	ch     chan Job
	mu     sync.Mutex
	closed bool
	wg     sync.WaitGroup
}

// NewMemory creates an in-memory queue with the given buffer size (default 64).
func NewMemory(buffer int) *Memory {
	if buffer <= 0 {
		buffer = 64
	}
	return &Memory{ch: make(chan Job, buffer)}
}

// Enqueue adds a job. Delayed jobs (Job.At in the future) sleep in a goroutine
// before entering the channel.
func (m *Memory) Enqueue(ctx context.Context, job Job) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return ErrClosed
	}
	m.mu.Unlock()

	if !job.At.IsZero() && time.Until(job.At) > 0 {
		delay := time.Until(job.At)
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
			m.mu.Lock()
			closed := m.closed
			m.mu.Unlock()
			if closed {
				return
			}
			select {
			case m.ch <- job:
			case <-ctx.Done():
			}
		}()
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case m.ch <- job:
		return nil
	}
}

// Run consumes jobs until ctx is done.
func (m *Memory) Run(ctx context.Context, h Handler) error {
	if h == nil {
		return errors.New("queue: nil handler")
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
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
func (m *Memory) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	close(m.ch)
	m.mu.Unlock()
	m.wg.Wait()
	return nil
}
