package session_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/session"
)

func TestMemoryStoreRoundTrip(t *testing.T) {
	store := session.NewMemory()
	ctx := t.Context()
	data := session.Data{
		Values:    map[string]any{"user": "alice"},
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := store.Save(ctx, "abc", data); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if got.Values["user"] != "alice" {
		t.Fatalf("got %#v", got.Values)
	}
	if err := store.Delete(ctx, "abc"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, "abc"); err != session.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestSessionMiddlewareFlash(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr, err := session.NewManager(session.Options{
		Name:     "test_sess",
		MaxAge:   time.Hour,
		Path:     "/",
		HTTPOnly: true,
		Secure:   false,
		SameSite: "lax",
		Store:    session.NewMemory(),
	})
	if err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	r.Use(mgr.Middleware())
	r.GET("/set", func(c *gin.Context) {
		s := session.MustGet(c)
		s.Set("uid", "42")
		s.SetFlash("notice", "welcome")
		c.Status(http.StatusOK)
	})
	r.GET("/get", func(c *gin.Context) {
		s := session.MustGet(c)
		uid := s.GetString("uid")
		flash, _ := s.PopFlash("notice")
		c.JSON(http.StatusOK, gin.H{"uid": uid, "flash": flash})
	})

	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/set", nil)
	r.ServeHTTP(w1, req1)
	cookie := w1.Result().Cookies()
	if len(cookie) == 0 {
		t.Fatal("expected session cookie")
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/get", nil)
	req2.AddCookie(cookie[0])
	r.ServeHTTP(w2, req2)
	body := w2.Body.String()
	if w2.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w2.Code, body)
	}
	if !contains(body, `"uid":"42"`) || !contains(body, `"flash":"welcome"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestNewRejectsUnregisteredRedis(t *testing.T) {
	_, err := session.New(session.Options{Driver: "redis"})
	if err == nil {
		t.Fatal("expected error when redis driver is not registered")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		})())
}
