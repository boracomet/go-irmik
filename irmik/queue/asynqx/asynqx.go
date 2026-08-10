// Package asynqx provides an asynq (Redis) backend for irmik/queue.
//
// Opt-in (keeps hibiken/asynq out of binaries that do not import this package):
//
//	import _ "github.com/boracomet/go-irmik/irmik/queue/asynqx"
//
//	q, err := queue.New(queue.Options{Driver: "asynq", RedisURL: "redis://localhost:6379/0"})
//
// Or open explicitly:
//
//	q, err := asynqx.Open(asynqx.Options{RedisURL: "redis://localhost:6379/0"})
//	go q.Run(ctx, handler)
package asynqx

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hibiken/asynq"

	"github.com/boracomet/go-irmik/irmik/queue"
)

var registerOnce sync.Once

func init() {
	Register()
}

// Register wires drivers "asynq" and "redis" into queue.New. Safe to call multiple times.
func Register() {
	registerOnce.Do(func() {
		fn := func(opts queue.Options) (queue.Queue, error) {
			return Open(Options{
				RedisURL:    opts.RedisURL,
				Concurrency: opts.Concurrency,
				Queue:       opts.QueueName,
			})
		}
		queue.Register("asynq", fn)
		queue.Register("redis", fn)
	})
}

// Options configures the asynq-backed queue.
type Options struct {
	// RedisURL is a redis:// or rediss:// URI (default redis://localhost:6379/0).
	RedisURL string
	// Concurrency is the number of concurrent workers (default 10).
	Concurrency int
	// Queue is the asynq queue name (default "default").
	Queue string
	// RedisConnOpt overrides RedisURL when set (advanced / tests).
	RedisConnOpt asynq.RedisConnOpt
}

// Queue is a queue.Queue backed by asynq.
type Queue struct {
	client  *asynq.Client
	server  *asynq.Server
	queue   string
	mu      sync.Mutex
	closed  bool
	started bool
}

var _ queue.Queue = (*Queue)(nil)

// Open builds an asynq-backed queue.Queue.
func Open(opts Options) (*Queue, error) {
	conn, err := redisConn(opts)
	if err != nil {
		return nil, err
	}
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}
	qname := strings.TrimSpace(opts.Queue)
	if qname == "" {
		qname = "default"
	}
	client := asynq.NewClient(conn)
	server := asynq.NewServer(conn, asynq.Config{
		Concurrency: concurrency,
		Queues:      map[string]int{qname: 1},
	})
	return &Queue{client: client, server: server, queue: qname}, nil
}

func redisConn(opts Options) (asynq.RedisConnOpt, error) {
	if opts.RedisConnOpt != nil {
		return opts.RedisConnOpt, nil
	}
	url := strings.TrimSpace(opts.RedisURL)
	if url == "" {
		url = "redis://localhost:6379/0"
	}
	conn, err := asynq.ParseRedisURI(url)
	if err != nil {
		return nil, fmt.Errorf("asynqx: parse redis url: %w", err)
	}
	return conn, nil
}

// Enqueue pushes a job. Job.Name becomes the asynq task type; Job.At maps to ProcessAt.
func (q *Queue) Enqueue(ctx context.Context, job queue.Job) error {
	q.mu.Lock()
	closed := q.closed
	q.mu.Unlock()
	if closed {
		return queue.ErrClosed
	}
	name := strings.TrimSpace(job.Name)
	if name == "" {
		return errors.New("asynqx: job name is required")
	}
	task := asynq.NewTask(name, job.Payload)
	opts := []asynq.Option{asynq.Queue(q.queue)}
	if !job.At.IsZero() && job.At.After(time.Now()) {
		opts = append(opts, asynq.ProcessAt(job.At))
	}
	_, err := q.client.EnqueueContext(ctx, task, opts...)
	return err
}

// Run starts asynq workers and blocks until ctx is cancelled.
func (q *Queue) Run(ctx context.Context, h queue.Handler) error {
	if h == nil {
		return errors.New("asynqx: nil handler")
	}
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return queue.ErrClosed
	}
	if q.started {
		q.mu.Unlock()
		return errors.New("asynqx: Run already started")
	}
	q.started = true
	q.mu.Unlock()

	if err := q.server.Start(handlerFunc(h)); err != nil {
		return err
	}
	<-ctx.Done()
	q.server.Shutdown()
	return ctx.Err()
}

// Close stops accepting jobs and shuts down the asynq client/server.
func (q *Queue) Close() error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return nil
	}
	q.closed = true
	started := q.started
	q.mu.Unlock()

	if started {
		q.server.Shutdown()
	}
	return q.client.Close()
}

type handlerFunc queue.Handler

func (h handlerFunc) ProcessTask(ctx context.Context, t *asynq.Task) error {
	return h(ctx, queue.Job{Name: t.Type(), Payload: t.Payload()})
}
