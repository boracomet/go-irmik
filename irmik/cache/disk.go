package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/boracomet/go-irmik/irmik/fsutil"
)

type diskStore struct {
	dir string
	mu  sync.Mutex
}

type diskEntry struct {
	Body        []byte `json:"body"`
	ContentType string `json:"contentType"`
	ExpiresAt   int64  `json:"expiresAt"` // unix nano; 0 = none
	StaleAt     int64  `json:"staleAt"`
}

// NewDisk returns a filesystem-backed Store under dir (default .irmik/cache).
func NewDisk(dir string) (Store, error) {
	if dir == "" {
		dir = ".irmik/cache"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("cache disk mkdir: %w", err)
	}
	return &diskStore{dir: dir}, nil
}

func (s *diskStore) path(key string) string {
	// Human-readable SafeFileName + short hash suffix to avoid collisions.
	sum := sha256.Sum256([]byte(key))
	short := hex.EncodeToString(sum[:8])
	safe := fsutil.SafeFileName(key)
	if len(safe) > 80 {
		safe = safe[:80]
	}
	return filepath.Join(s.dir, safe+"_"+short+".json")
}

func (s *diskStore) Get(_ context.Context, key string) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path(key))
	if err != nil {
		if os.IsNotExist(err) {
			return Entry{}, ErrMiss
		}
		return Entry{}, err
	}

	var de diskEntry
	if err := json.Unmarshal(data, &de); err != nil {
		return Entry{}, err
	}
	e := entryFromDisk(de)
	if e.Expired() {
		_ = os.Remove(s.path(key))
		return Entry{}, ErrMiss
	}
	return e, nil
}

func (s *diskStore) Set(_ context.Context, key string, entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	de := diskFromEntry(entry)
	data, err := json.Marshal(de)
	if err != nil {
		return err
	}
	tmp := s.path(key) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path(key))
}

func (s *diskStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(s.path(key))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *diskStore) Clear(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		_ = os.Remove(filepath.Join(s.dir, e.Name()))
	}
	return nil
}

func (s *diskStore) Close() error { return nil }

func diskFromEntry(e Entry) diskEntry {
	de := diskEntry{
		Body:        e.Body,
		ContentType: e.ContentType,
	}
	if !e.ExpiresAt.IsZero() {
		de.ExpiresAt = e.ExpiresAt.UnixNano()
	}
	if !e.StaleAt.IsZero() {
		de.StaleAt = e.StaleAt.UnixNano()
	}
	return de
}

func entryFromDisk(de diskEntry) Entry {
	e := Entry{
		Body:        de.Body,
		ContentType: de.ContentType,
	}
	if de.ExpiresAt != 0 {
		e.ExpiresAt = unixNano(de.ExpiresAt)
	}
	if de.StaleAt != 0 {
		e.StaleAt = unixNano(de.StaleAt)
	}
	return e
}
