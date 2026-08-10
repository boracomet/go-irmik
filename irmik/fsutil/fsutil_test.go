package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeFileName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"GET|/blog/hello|en", "GET__blog_hello_en"},
		{"/about:en", "about_en"},
		{"", "_empty"},
		{"///", "_empty"},
		{`a\b`, "a_b"},
	}
	for _, tt := range tests {
		if got := SafeFileName(tt.in); got != tt.want {
			t.Errorf("SafeFileName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestJoinSafe(t *testing.T) {
	got := JoinSafe("/tmp/cache", "GET|/x|en", "html")
	want := filepath.Join("/tmp/cache", "GET__x_en.html")
	if got != want {
		t.Fatalf("JoinSafe = %q, want %q", got, want)
	}
}

func TestEnsureDirAndWalkHTML(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "partials")
	if err := EnsureDir(sub); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "nav.html"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := WalkHTML(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "partials/nav.html" {
		t.Fatalf("WalkHTML = %#v", files)
	}
	if !Exists(sub) {
		t.Fatal("expected Exists")
	}
}

func TestWalkHTMLMissing(t *testing.T) {
	files, err := WalkHTML(filepath.Join(t.TempDir(), "nope"))
	if err != nil || files != nil {
		t.Fatalf("got %#v %v", files, err)
	}
}
