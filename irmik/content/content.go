// Package content loads Markdown collections with YAML/TOML/JSON frontmatter.
package content

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/adrg/frontmatter"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// Document is a Markdown file with typed frontmatter and rendered HTML body.
type Document[T any] struct {
	Collection string
	Slug       string
	Path       string // absolute path on disk
	Meta       T
	Body       string // HTML
	Raw        string // Markdown body without frontmatter
}

// Entry is an untyped document (frontmatter as map).
type Entry = Document[map[string]any]

// Store holds loaded collections from a content root directory.
type Store struct {
	root string
	md   goldmark.Markdown
	// collection -> slug -> file absolute path
	files map[string]map[string]string
}

// Load scans dir for content/<collection>/**/*.md and indexes them.
// dir itself is the content root (e.g. "./content").
func Load(dir string) (*Store, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("content: resolve %q: %w", dir, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("content: open %q: %w", abs, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("content: %q is not a directory", abs)
	}

	s := &Store{
		root:  abs,
		md:    defaultMarkdown(),
		files: make(map[string]map[string]string),
	}
	err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		rel, err := filepath.Rel(abs, path)
		if err != nil {
			return err
		}
		parts := splitPath(rel)
		if len(parts) < 2 {
			// Markdown must live under a collection subdirectory.
			return nil
		}
		collection := parts[0]
		slugParts := append([]string(nil), parts[1:]...)
		last := slugParts[len(slugParts)-1]
		slugParts[len(slugParts)-1] = strings.TrimSuffix(last, filepath.Ext(last))
		slug := strings.Join(slugParts, "/")
		if _, ok := s.files[collection]; !ok {
			s.files[collection] = make(map[string]string)
		}
		s.files[collection][slug] = path
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("content: walk %q: %w", abs, err)
	}
	return s, nil
}

// Root returns the absolute content root directory.
func (s *Store) Root() string {
	return s.root
}

// Collections returns sorted collection names.
func (s *Store) Collections() []string {
	names := make([]string, 0, len(s.files))
	for name := range s.files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// List returns all documents in a collection with map frontmatter, sorted by slug.
func (s *Store) List(collection string) ([]Entry, error) {
	return List[map[string]any](s, collection)
}

// Get returns one document by collection and slug with map frontmatter.
func (s *Store) Get(collection, slug string) (*Entry, error) {
	return Get[map[string]any](s, collection, slug)
}

// List returns typed documents for a collection, sorted by slug.
func List[T any](s *Store, collection string) ([]Document[T], error) {
	if s == nil {
		return nil, fmt.Errorf("content: nil store")
	}
	slugs, ok := s.files[collection]
	if !ok {
		return nil, fmt.Errorf("content: unknown collection %q", collection)
	}
	keys := make([]string, 0, len(slugs))
	for slug := range slugs {
		keys = append(keys, slug)
	}
	sort.Strings(keys)
	out := make([]Document[T], 0, len(keys))
	for _, slug := range keys {
		doc, err := loadDocument[T](s, collection, slug, slugs[slug])
		if err != nil {
			return nil, err
		}
		out = append(out, *doc)
	}
	return out, nil
}

// Get returns a typed document by collection and slug.
func Get[T any](s *Store, collection, slug string) (*Document[T], error) {
	if s == nil {
		return nil, fmt.Errorf("content: nil store")
	}
	slugs, ok := s.files[collection]
	if !ok {
		return nil, fmt.Errorf("content: unknown collection %q", collection)
	}
	path, ok := slugs[slug]
	if !ok {
		return nil, fmt.Errorf("content: %s/%s not found", collection, slug)
	}
	return loadDocument[T](s, collection, slug, path)
}

// Decode parses frontmatter from raw file bytes into dst and returns the Markdown body.
func Decode(data []byte, dst any) (body []byte, err error) {
	rest, err := frontmatter.Parse(bytes.NewReader(data), dst)
	if err != nil {
		return nil, fmt.Errorf("content: frontmatter: %w", err)
	}
	return rest, nil
}

// RenderMarkdown converts Markdown to HTML using the package default engine.
func RenderMarkdown(src []byte) (string, error) {
	var buf bytes.Buffer
	if err := defaultMarkdown().Convert(src, &buf); err != nil {
		return "", fmt.Errorf("content: markdown: %w", err)
	}
	return buf.String(), nil
}

func loadDocument[T any](s *Store, collection, slug, path string) (*Document[T], error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("content: read %s: %w", path, err)
	}
	var meta T
	rest, err := Decode(data, &meta)
	if err != nil {
		return nil, fmt.Errorf("content: parse %s: %w", path, err)
	}
	htmlBody, err := s.render(rest)
	if err != nil {
		return nil, fmt.Errorf("content: render %s: %w", path, err)
	}
	return &Document[T]{
		Collection: collection,
		Slug:       slug,
		Path:       path,
		Meta:       meta,
		Body:       htmlBody,
		Raw:        string(rest),
	}, nil
}

func (s *Store) render(src []byte) (string, error) {
	var buf bytes.Buffer
	if err := s.md.Convert(src, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func defaultMarkdown() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)
}

func splitPath(rel string) []string {
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		out = append(out, p)
	}
	return out
}
