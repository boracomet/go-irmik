package cache

import (
	"context"
	"sync"
)

type memoryStore struct {
	mu   sync.RWMutex
	data map[string]Entry
}

// NewMemory returns an in-process Store.
func NewMemory() Store {
	return &memoryStore{data: make(map[string]Entry)}
}

func (s *memoryStore) Get(_ context.Context, key string) (Entry, error) {
	s.mu.RLock()
	e, ok := s.data[key]
	s.mu.RUnlock()
	if !ok {
		return Entry{}, ErrMiss
	}
	if e.Expired() {
		_ = s.Delete(context.Background(), key)
		return Entry{}, ErrMiss
	}
	return e, nil
}

func (s *memoryStore) Set(_ context.Context, key string, entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = entry
	return nil
}

func (s *memoryStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

func (s *memoryStore) Clear(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string]Entry)
	return nil
}

func (s *memoryStore) Close() error { return nil }
