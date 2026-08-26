package auth

import (
	"testing"
	"time"
)

func TestMemoryRefreshStoreTTLAndRevoke(t *testing.T) {
	s := NewMemoryRefreshStore(50 * time.Millisecond)
	if err := s.Put("j1", "u1", time.Now().Add(30*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	uid, ok := s.Consume("j1")
	if !ok || uid != "u1" {
		t.Fatalf("consume fresh: uid=%q ok=%v", uid, ok)
	}
	if _, ok := s.Consume("j1"); ok {
		t.Fatal("one-time consume should not succeed twice")
	}

	if err := s.Put("j2", "u1", time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	s.RevokeUser("u1")
	if !s.Revoked("u1") {
		t.Fatal("expected user revoked")
	}
	if _, ok := s.Consume("j2"); ok {
		t.Fatal("revoked user token was consumed")
	}

	if err := s.Put("j3", "u2", time.Now().Add(20*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	if _, ok := s.Consume("j3"); ok {
		t.Fatal("expired token was consumed")
	}
}
