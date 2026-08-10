package asynqx

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"

	"github.com/boracomet/go-irmik/irmik/queue"
)

func TestOpenAndRegister(t *testing.T) {
	mr := miniredis.RunT(t)
	q, err := Open(Options{
		RedisConnOpt: asynq.RedisClientOpt{Addr: mr.Addr()},
		Concurrency:  1,
		Queue:        "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()

	if err := q.Enqueue(context.Background(), queue.Job{}); err == nil {
		t.Fatal("expected error for empty job name")
	}

	Register() // idempotent
	got, err := queue.New(queue.Options{
		Driver:   "asynq",
		RedisURL: "redis://" + mr.Addr() + "/0",
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = got.Close()
}

func TestEnqueueRun(t *testing.T) {
	mr := miniredis.RunT(t)
	q, err := Open(Options{
		RedisConnOpt: asynq.RedisClientOpt{Addr: mr.Addr()},
		Concurrency:  1,
		Queue:        "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var n atomic.Int32
	done := make(chan struct{})
	go func() {
		_ = q.Run(ctx, func(ctx context.Context, job queue.Job) error {
			if job.Name == "ping" && string(job.Payload) == "pong" {
				n.Add(1)
				close(done)
			}
			return nil
		})
	}()

	// Give the server a moment to start before enqueueing.
	time.Sleep(50 * time.Millisecond)
	if err := q.Enqueue(ctx, queue.Job{Name: "ping", Payload: []byte("pong")}); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for job")
	}
	if n.Load() != 1 {
		t.Fatalf("n=%d", n.Load())
	}
	cancel()
}

func TestQueueNewRequiresRegister(t *testing.T) {
	// Ensure the error path documents the blank-import when driver missing.
	// asynqx init already registered in this package's tests; call via a fresh check
	// on the unknown driver path instead.
	_, err := queue.New(queue.Options{Driver: "nope"})
	if err == nil {
		t.Fatal("expected unknown driver error")
	}
}
