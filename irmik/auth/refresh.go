package auth

import (
	"sync"
	"time"
)

// RefreshStore tracks one-time refresh tokens and user-level revocation.
//
// The default MemoryRefreshStore is process-local: it does not survive restarts
// and does not coordinate across replicas. Production multi-instance apps should
// implement this with Redis or a database.
type RefreshStore interface {
	// Put stores refresh token id (JWT jti) for userID until expiresAt.
	Put(id, userID string, expiresAt time.Time) error
	// Consume atomically loads and deletes id. ok is false if missing, expired, or revoked.
	Consume(id string) (userID string, ok bool)
	// RevokeUser invalidates outstanding refresh tokens for userID until their TTL would have elapsed.
	RevokeUser(userID string)
	// Revoked reports whether userID is currently revoked.
	Revoked(userID string) bool
}

type memoryRefresh struct {
	UserID    string
	ExpiresAt time.Time
}

// MemoryRefreshStore is an in-process RefreshStore with TTL and lazy GC.
type MemoryRefreshStore struct {
	mu           sync.Mutex
	ttl          time.Duration
	tokens       map[string]memoryRefresh
	revokedUntil map[string]time.Time
	lastGC       time.Time
}

// NewMemoryRefreshStore returns a process-local store. ttl is used for
// revocation expiry (default 7d) and GC cadence.
func NewMemoryRefreshStore(ttl time.Duration) *MemoryRefreshStore {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &MemoryRefreshStore{
		ttl:          ttl,
		tokens:       make(map[string]memoryRefresh),
		revokedUntil: make(map[string]time.Time),
	}
}

// Put implements RefreshStore.
func (s *MemoryRefreshStore) Put(id, userID string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gcLocked(time.Now())
	s.tokens[id] = memoryRefresh{UserID: userID, ExpiresAt: expiresAt}
	return nil
}

// Consume implements RefreshStore.
func (s *MemoryRefreshStore) Consume(id string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.gcLocked(now)
	rec, ok := s.tokens[id]
	if !ok {
		return "", false
	}
	delete(s.tokens, id)
	if now.After(rec.ExpiresAt) {
		return "", false
	}
	if until, revoked := s.revokedUntil[rec.UserID]; revoked && now.Before(until) {
		return "", false
	}
	return rec.UserID, true
}

// RevokeUser implements RefreshStore.
func (s *MemoryRefreshStore) RevokeUser(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.gcLocked(now)
	s.revokedUntil[userID] = now.Add(s.ttl)
	for id, rec := range s.tokens {
		if rec.UserID == userID {
			delete(s.tokens, id)
		}
	}
}

// Revoked implements RefreshStore.
func (s *MemoryRefreshStore) Revoked(userID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.gcLocked(now)
	until, ok := s.revokedUntil[userID]
	return ok && now.Before(until)
}

func (s *MemoryRefreshStore) gcLocked(now time.Time) {
	if !s.lastGC.IsZero() && now.Sub(s.lastGC) < time.Minute && len(s.tokens) < 1024 {
		return
	}
	s.lastGC = now
	for id, rec := range s.tokens {
		if now.After(rec.ExpiresAt) {
			delete(s.tokens, id)
		}
	}
	for uid, until := range s.revokedUntil {
		if !now.Before(until) {
			delete(s.revokedUntil, uid)
		}
	}
}
