// Package secrets provides a small secret Provider interface with env and file backends.
package secrets

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// Provider resolves named secrets.
type Provider interface {
	Get(name string) (string, error)
}

// Env reads secrets from environment variables.
// Optional Prefix is prepended (e.g. "IRMIK_" → IRMIK_JWT_SECRET for "JWT_SECRET").
type Env struct {
	Prefix string
}

// Get returns the environment value for name.
func (e Env) Get(name string) (string, error) {
	key := e.Prefix + name
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return "", fmt.Errorf("secrets: env %q not set", key)
	}
	return v, nil
}

// File reads a secret from a file path. name is ignored; Path is the file.
// Prefer Map or Multi for multiple named file secrets.
type File struct {
	Path string
}

// Get reads and trims the file contents.
func (f File) Get(name string) (string, error) {
	_ = name
	b, err := os.ReadFile(f.Path)
	if err != nil {
		return "", fmt.Errorf("secrets: read %s: %w", f.Path, err)
	}
	return strings.TrimSpace(string(b)), nil
}

// Map is an in-memory Provider (tests / explicit wiring).
type Map map[string]string

// Get returns the mapped value.
func (m Map) Get(name string) (string, error) {
	v, ok := m[name]
	if !ok || v == "" {
		return "", fmt.Errorf("secrets: %q not found", name)
	}
	return v, nil
}

// Files maps secret names to file paths (e.g. Kubernetes-style).
type Files map[string]string

// Get reads the file for name.
func (f Files) Get(name string) (string, error) {
	path, ok := f[name]
	if !ok {
		return "", fmt.Errorf("secrets: no file mapping for %q", name)
	}
	return (File{Path: path}).Get(name)
}

// Multi tries providers in order until one succeeds.
type Multi struct {
	Providers []Provider
}

// Get returns the first successful value.
func (m Multi) Get(name string) (string, error) {
	var last error
	for _, p := range m.Providers {
		v, err := p.Get(name)
		if err == nil {
			return v, nil
		}
		last = err
	}
	if last == nil {
		return "", fmt.Errorf("secrets: %q not found", name)
	}
	return "", last
}

// Cached wraps a Provider with an in-memory cache.
type Cached struct {
	Inner Provider
	mu    sync.Mutex
	cache map[string]string
}

// Get returns a cached or freshly loaded secret.
func (c *Cached) Get(name string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cache == nil {
		c.cache = map[string]string{}
	}
	if v, ok := c.cache[name]; ok {
		return v, nil
	}
	v, err := c.Inner.Get(name)
	if err != nil {
		return "", err
	}
	c.cache[name] = v
	return v, nil
}
