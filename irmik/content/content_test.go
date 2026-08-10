package content_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boracomet/go-irmik/irmik/content"
)

type postMeta struct {
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Draft       bool     `yaml:"draft"`
	Tags        []string `yaml:"tags"`
}

func TestLoadFrontmatterAndBody(t *testing.T) {
	root := t.TempDir()
	posts := filepath.Join(root, "posts")
	if err := os.MkdirAll(filepath.Join(posts, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\n" +
		"title: Hello World\n" +
		"description: A sample post\n" +
		"draft: false\n" +
		"tags:\n" +
		"  - go\n" +
		"  - irmik\n" +
		"---\n" +
		"# Hello\n\n" +
		"This is **bold** text.\n"
	if err := os.WriteFile(filepath.Join(posts, "hello.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := "---\ntitle: Nested\n---\n\nNested body.\n"
	if err := os.WriteFile(filepath.Join(posts, "nested", "deep.md"), []byte(nested), 0o644); err != nil {
		t.Fatal(err)
	}
	// Markdown at content root should be ignored (no collection).
	if err := os.WriteFile(filepath.Join(root, "readme.md"), []byte("---\ntitle: x\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := content.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := store.Collections(); len(got) != 1 || got[0] != "posts" {
		t.Fatalf("Collections = %v, want [posts]", got)
	}

	doc, err := content.Get[postMeta](store, "posts", "hello")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if doc.Meta.Title != "Hello World" {
		t.Errorf("Title = %q", doc.Meta.Title)
	}
	if doc.Meta.Description != "A sample post" {
		t.Errorf("Description = %q", doc.Meta.Description)
	}
	if doc.Meta.Draft {
		t.Error("Draft should be false")
	}
	if len(doc.Meta.Tags) != 2 || doc.Meta.Tags[0] != "go" || doc.Meta.Tags[1] != "irmik" {
		t.Errorf("Tags = %#v", doc.Meta.Tags)
	}
	if !strings.Contains(doc.Body, "<strong>bold</strong>") {
		t.Errorf("Body missing bold markup: %q", doc.Body)
	}
	if !strings.Contains(doc.Body, "<h1") {
		t.Errorf("Body missing h1: %q", doc.Body)
	}
	if !strings.Contains(doc.Raw, "**bold**") {
		t.Errorf("Raw should keep markdown: %q", doc.Raw)
	}

	list, err := content.List[postMeta](store, "posts")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List len = %d, want 2", len(list))
	}
	// Sorted by slug: hello, nested/deep
	if list[0].Slug != "hello" || list[1].Slug != "nested/deep" {
		t.Fatalf("slugs = %q, %q", list[0].Slug, list[1].Slug)
	}

	entry, err := store.Get("posts", "nested/deep")
	if err != nil {
		t.Fatalf("store.Get: %v", err)
	}
	if entry.Meta["title"] != "Nested" {
		t.Errorf("map title = %#v", entry.Meta["title"])
	}
}

func TestDecodeHelper(t *testing.T) {
	data := []byte("---\ntitle: X\n---\n\nBody *here*.\n")
	var meta postMeta
	body, err := content.Decode(data, &meta)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "X" {
		t.Errorf("title = %q", meta.Title)
	}
	html, err := content.RenderMarkdown(body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "<em>here</em>") {
		t.Errorf("html = %q", html)
	}
}

func TestUnknownCollection(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "posts"), 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := content.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("missing", "x"); err == nil {
		t.Fatal("expected error")
	}
}
