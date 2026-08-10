package sse

import (
	"sync"
)

// Hub fans out Events to subscribed Streams (or raw channels).
type Hub struct {
	mu   sync.RWMutex
	subs map[chan Event]struct{}
}

// NewHub creates an empty broadcast hub.
func NewHub() *Hub {
	return &Hub{subs: make(map[chan Event]struct{})}
}

// Subscribe registers a buffered channel. Call Unsubscribe when done.
func (h *Hub) Subscribe(buf int) chan Event {
	if buf < 1 {
		buf = 16
	}
	ch := make(chan Event, buf)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes ch and closes it.
func (h *Hub) Unsubscribe(ch chan Event) {
	h.mu.Lock()
	if _, ok := h.subs[ch]; ok {
		delete(h.subs, ch)
		close(ch)
	}
	h.mu.Unlock()
}

// Broadcast sends e to all subscribers. Slow subscribers drop the event
// when their buffer is full (non-blocking send).
func (h *Hub) Broadcast(e Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// Len returns the number of active subscribers.
func (h *Hub) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs)
}

// Attach writes hub events to s until the stream context is done.
func (h *Hub) Attach(s *Stream) {
	ch := h.Subscribe(32)
	defer h.Unsubscribe(ch)
	for {
		select {
		case <-s.Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			if err := s.Write(e); err != nil {
				return
			}
		}
	}
}
