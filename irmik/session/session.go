package session

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const contextKey = "irmik_session"

// Manager loads and persists cookie-backed sessions.
type Manager struct {
	opts  Options
	store Store
}

// NewManager constructs a Manager. If opts.Store is nil, New(opts) is used.
func NewManager(opts Options) (*Manager, error) {
	opts = normalizeOptions(opts)
	store, err := New(opts)
	if err != nil {
		return nil, err
	}
	return &Manager{opts: opts, store: store}, nil
}

// Store returns the underlying session store.
func (m *Manager) Store() Store { return m.store }

// Options returns a copy of manager options.
func (m *Manager) Options() Options { return m.opts }

// Close closes the underlying store.
func (m *Manager) Close() error {
	if m.store == nil {
		return nil
	}
	return m.store.Close()
}

// Middleware loads the session for each request and saves when dirty.
func (m *Manager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		sess := m.load(c)
		c.Set(contextKey, sess)
		c.Next()
		if sess != nil && sess.dirty {
			_ = sess.persist(c)
		}
	}
}

// Get returns the request Session (or nil if middleware not installed).
func Get(c *gin.Context) *Session {
	v, ok := c.Get(contextKey)
	if !ok {
		return nil
	}
	s, _ := v.(*Session)
	return s
}

// MustGet returns the session or panics.
func MustGet(c *gin.Context) *Session {
	s := Get(c)
	if s == nil {
		panic("session: middleware not installed")
	}
	return s
}

func (m *Manager) load(c *gin.Context) *Session {
	sess := &Session{
		mgr:      m,
		Values:   make(map[string]any),
		Flash:    make(map[string]any),
		outFlash: make(map[string]any),
	}
	cookie, err := c.Cookie(m.opts.Name)
	if err != nil || cookie == "" {
		sess.isNew = true
		return sess
	}
	data, err := m.store.Get(c.Request.Context(), cookie)
	if err != nil {
		sess.isNew = true
		return sess
	}
	sess.ID = cookie
	if data.Values != nil {
		sess.Values = data.Values
	}
	// Incoming flash is available this request only; clear it from the store on save.
	if len(data.Flash) > 0 {
		sess.Flash = data.Flash
		sess.dirty = true
	}
	sess.ExpiresAt = data.ExpiresAt
	return sess
}

// Session is the per-request session view.
type Session struct {
	ID        string
	Values    map[string]any
	Flash     map[string]any // readable flash from previous request
	ExpiresAt time.Time

	mgr      *Manager
	mu       sync.Mutex
	outFlash map[string]any // flash to persist for the next request
	dirty    bool
	isNew    bool
}

// Get returns a value from the session.
func (s *Session) Get(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.Values[key]
	return v, ok
}

// GetString returns a string value.
func (s *Session) GetString(key string) string {
	v, ok := s.Get(key)
	if !ok {
		return ""
	}
	str, _ := v.(string)
	return str
}

// Set stores a value.
func (s *Session) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Values == nil {
		s.Values = make(map[string]any)
	}
	s.Values[key] = value
	s.dirty = true
}

// Delete removes a value.
func (s *Session) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Values, key)
	s.dirty = true
}

// SetFlash stores a one-shot flash message for the next request.
func (s *Session) SetFlash(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.outFlash == nil {
		s.outFlash = make(map[string]any)
	}
	s.outFlash[key] = value
	s.dirty = true
}

// PopFlash returns and clears an incoming flash value for this request.
func (s *Session) PopFlash(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.Flash[key]
	if ok {
		delete(s.Flash, key)
	}
	return v, ok
}

// Clear removes all values and marks dirty.
func (s *Session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Values = make(map[string]any)
	s.Flash = make(map[string]any)
	s.outFlash = make(map[string]any)
	s.dirty = true
}

// Save persists immediately.
func (s *Session) Save(c *gin.Context) error {
	return s.persist(c)
}

// Regenerate issues a new session id (call after login).
func (s *Session) Regenerate(c *gin.Context) error {
	s.mu.Lock()
	old := s.ID
	s.mu.Unlock()
	if old != "" {
		_ = s.mgr.store.Delete(c.Request.Context(), old)
	}
	s.mu.Lock()
	s.ID = ""
	s.isNew = true
	s.dirty = true
	s.mu.Unlock()
	return s.persist(c)
}

// Destroy deletes the session and clears the cookie.
func (s *Session) Destroy(c *gin.Context) error {
	s.mu.Lock()
	id := s.ID
	s.Values = make(map[string]any)
	s.Flash = make(map[string]any)
	s.outFlash = make(map[string]any)
	s.ID = ""
	s.dirty = false
	s.mu.Unlock()
	if id != "" {
		_ = s.mgr.store.Delete(c.Request.Context(), id)
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     s.mgr.opts.Name,
		Value:    "",
		Path:     s.mgr.opts.Path,
		Domain:   s.mgr.opts.Domain,
		MaxAge:   -1,
		HttpOnly: s.mgr.opts.HTTPOnly,
		Secure:   s.mgr.opts.Secure,
		SameSite: sameSiteMode(s.mgr.opts.SameSite),
	})
	return nil
}

func (s *Session) persist(c *gin.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ID == "" {
		id, err := newID()
		if err != nil {
			return err
		}
		s.ID = id
	}
	exp := time.Now().Add(s.mgr.opts.MaxAge)
	s.ExpiresAt = exp

	data := Data{
		Values:    cloneMap(s.Values),
		Flash:     cloneMap(s.outFlash),
		ExpiresAt: exp,
	}
	if err := s.mgr.store.Save(c.Request.Context(), s.ID, data); err != nil {
		return err
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     s.mgr.opts.Name,
		Value:    s.ID,
		Path:     s.mgr.opts.Path,
		Domain:   s.mgr.opts.Domain,
		MaxAge:   int(s.mgr.opts.MaxAge.Seconds()),
		HttpOnly: s.mgr.opts.HTTPOnly,
		Secure:   s.mgr.opts.Secure,
		SameSite: sameSiteMode(s.mgr.opts.SameSite),
		Expires:  exp,
	})
	s.dirty = false
	s.isNew = false
	s.outFlash = make(map[string]any)
	return nil
}

func newID() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
