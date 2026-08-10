// Package fsutil provides small filesystem helpers shared by build, cache, and tooling.
// Patterns inspired by StatiGo (MIT); reimplemented for Irmik.
package fsutil

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// EnsureDir creates dir and parents with 0755 if missing.
func EnsureDir(dir string) error {
	if dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

// SafeFileName turns an arbitrary cache/route key into a single path segment
// safe for use under a cache or export directory (no separators).
// Example: "GET|/blog/hello|en" → "GET__blog_hello_en"
func SafeFileName(key string) string {
	name := strings.ReplaceAll(key, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "|", "_")
	name = strings.Trim(name, "._")
	if name == "" {
		return "_empty"
	}
	return name
}

// JoinSafe joins base with a SafeFileName(key) and optional extension (with or without dot).
func JoinSafe(base, key, ext string) string {
	name := SafeFileName(key)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return filepath.Join(base, name+ext)
}

// WalkHTML walks root and returns paths of .html files relative to root.
// Missing root yields (nil, nil).
func WalkHTML(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	var out []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".html") {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	return out, err
}

// Exists reports whether path exists.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
