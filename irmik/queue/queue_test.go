package queue

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestMemoryRun(t *testing.T) {
	q := NewMemory(8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var n atomic.Int32
	done := make(chan struct{})
	go func() {
		_ = q.Run(ctx, func(ctx context.Context, job Job) error {
			n.Add(1)
			if job.Name == "last" {
				close(done)
			}
			return nil
		})
	}()

	_ = q.Enqueue(ctx, Job{Name: "a", Payload: []byte("1")})
	_ = q.Enqueue(ctx, Job{Name: "last", Payload: []byte("2")})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	if n.Load() != 2 {
		t.Fatalf("n=%d", n.Load())
	}
	cancel()
	_ = q.Close()
}
