package queue

import (
	"testing"
)

func TestNewMemoryDriver(t *testing.T) {
	q, err := New(Options{Driver: "memory", Buffer: 4})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*Memory); !ok {
		t.Fatalf("got %T", q)
	}
	_ = q.Close()
}

func TestNewMissingAsynqDriver(t *testing.T) {
	_, err := New(Options{Driver: "asynq"})
	if err == nil {
		t.Fatal("expected error without asynqx registered")
	}
}
