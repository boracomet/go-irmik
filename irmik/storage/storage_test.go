package storage

import (
	"bytes"
	"context"
	"io"
	"testing"
)

func TestLocalRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, err := OpenLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	obj, err := s.Put(ctx, "a/b.txt", bytes.NewReader([]byte("hi")), "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	if obj.Size != 2 || obj.ContentType != "text/plain" {
		t.Fatalf("obj = %+v", obj)
	}
	rc, got, err := s.Get(ctx, "a/b.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rc.Close() }()
	data, _ := io.ReadAll(rc)
	if string(data) != "hi" || got.ContentType != "text/plain" {
		t.Fatalf("get = %q %+v", data, got)
	}
	if err := s.Delete(ctx, "a/b.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Stat(ctx, "a/b.txt"); err != ErrNotFound {
		t.Fatalf("stat after delete: %v", err)
	}
}

func TestLocalRejectsTraversal(t *testing.T) {
	s, err := OpenLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Put(context.Background(), "../x", bytes.NewReader([]byte("x")), "")
	if err == nil {
		t.Fatal("expected error")
	}
}
