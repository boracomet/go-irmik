package router

import (
	"path/filepath"
	"testing"
)

func TestFilePathToGin(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", "/"},
		{".", "/"},
		{"about", "/about"},
		{"blog/[slug]", "/blog/:slug"},
		{"blog/[slug]/edit", "/blog/:slug/edit"},
		{"docs/[...slug]", "/docs/*slug"},
		{"(marketing)/about", "/about"},
		{filepath.FromSlash("blog/[slug]"), "/blog/:slug"},
	}
	for _, tt := range tests {
		got := FilePathToGin(tt.in)
		if got != tt.want {
			t.Errorf("FilePathToGin(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFilePathToGinNestedDynamic(t *testing.T) {
	got := FilePathToGin("shop/[category]/[item]")
	want := "/shop/:category/:item"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
