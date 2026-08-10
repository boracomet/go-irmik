package session

import (
	"context"
	"sync"
	"time"
)

type memoryStore struct {
	mu   sync.RWMutex
	data map[string]Data
}

// NewMemory returns an in-process session Store.
func NewMemory() Store {
	return &memoryStore{data: make(map[string]Data)}
}

func (s *memoryStore) Get(_ context.Context, id string) (Data, error) {
	s.mu.RLock()
	d, ok := s.data[id]
	s.mu.RUnlock()
	if !ok {
		return Data{}, ErrNotFound
	}
	if !d.ExpiresAt.IsZero() && time.Now().After(d.ExpiresAt) {
		_ = s.Delete(context.Background(), id)
		return Data{}, ErrNotFound
	}
	return cloneData(d), nil
}

func (s *memoryStore) Save(_ context.Context, id string, data Data) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[id] = cloneData(data)
	return nil
}

func (s *memoryStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, id)
	return nil
}

func (s *memoryStore) Close() error { return nil }

func cloneData(d Data) Data {
	out := Data{ExpiresAt: d.ExpiresAt}
	if d.Values != nil {
		out.Values = make(map[string]any, len(d.Values))
		for k, v := range d.Values {
			out.Values[k] = v
		}
	}
	if d.Flash != nil {
		out.Flash = make(map[string]any, len(d.Flash))
		for k, v := range d.Flash {
			out.Flash[k] = v
		}
	}
	return out
}
