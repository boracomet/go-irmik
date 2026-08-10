package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestEvery(t *testing.T) {
	s := New()
	var n atomic.Int32
	if err := s.Add(Job{
		Name:  "tick",
		Every: 20 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			n.Add(1)
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	_ = s.Run(ctx)
	if n.Load() < 2 {
		t.Fatalf("expected >=2 ticks, got %d", n.Load())
	}
}

func TestParseCron(t *testing.T) {
	spec, err := parseCron("*/5 * * * *")
	if err != nil {
		t.Fatal(err)
	}
	if !matchField(spec.min, 10) || matchField(spec.min, 7) {
		t.Fatal("step match failed")
	}
	_, err = parseCron("bad")
	if err == nil {
		t.Fatal("expected error")
	}
}
