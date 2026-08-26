package cache

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMemoryStore(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	_, err := s.Get(ctx, "missing")
	if !errors.Is(err, ErrMiss) {
		t.Fatalf("expected ErrMiss, got %v", err)
	}

	entry := Entry{
		Body:        []byte("hello"),
		ContentType: "text/html",
		ExpiresAt:   time.Now().Add(time.Minute),
	}
	if err := s.Set(ctx, "k", entry); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Body) != "hello" {
		t.Fatalf("body=%q", got.Body)
	}

	expired := Entry{Body: []byte("x"), ExpiresAt: time.Now().Add(-time.Second)}
	_ = s.Set(ctx, "old", expired)
	if _, err := s.Get(ctx, "old"); !errors.Is(err, ErrMiss) {
		t.Fatalf("expected miss for expired, got %v", err)
	}

	_ = s.Clear(ctx)
	if _, err := s.Get(ctx, "k"); !errors.Is(err, ErrMiss) {
		t.Fatalf("expected miss after clear")
	}
	_ = s.Close()
}

func TestDiskStore(t *testing.T) {
	dir := t.TempDir()
	s, err := NewDisk(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	entry := Entry{
		Body:        []byte("<html/>"),
		ContentType: "text/html",
		StaleAt:     time.Now().Add(30 * time.Second),
		ExpiresAt:   time.Now().Add(time.Minute),
	}
	if err := s.Set(ctx, "page", entry); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, "page")
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Body) != "<html/>" || got.ContentType != "text/html" {
		t.Fatalf("unexpected entry: %+v", got)
	}
	if got.Expired() || !got.StaleAt.After(time.Now()) {
		t.Fatalf("timestamps not restored: expires=%v stale=%v", got.ExpiresAt, got.StaleAt)
	}

	if err := s.Delete(ctx, "page"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, "page"); !errors.Is(err, ErrMiss) {
		t.Fatalf("expected miss after delete")
	}

	_ = s.Set(ctx, "a", Entry{Body: []byte("1")})
	_ = s.Clear(ctx)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			t.Fatalf("expected clear to remove json files, found %s", e.Name())
		}
	}
	_ = s.Close()
}

func TestKey(t *testing.T) {
	if got := Key("get", "/blog", "en"); got != "GET|/blog|en" {
		t.Fatalf("got %q", got)
	}
	if got := Key("", "", ""); got != "GET|/|en" {
		t.Fatalf("defaults got %q", got)
	}
	if got := Key("HEAD", "/blog", "en"); got != "GET|/blog|en" {
		t.Fatalf("HEAD should share GET key, got %q", got)
	}
}

func TestNewDrivers(t *testing.T) {
	s, err := New(Options{Driver: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	dir := t.TempDir()
	s, err = New(Options{Driver: "disk", DiskDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	_, err = New(Options{Driver: "unknown"})
	if err == nil {
		t.Fatal("expected error for unknown driver")
	}

	_, err = New(Options{Driver: "redis"})
	if err == nil {
		t.Fatal("expected error when redis driver is not registered")
	}
}
