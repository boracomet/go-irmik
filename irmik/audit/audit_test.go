package audit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/audit"
	"github.com/boracomet/go-irmik/irmik/auth"
)

func TestMemory(t *testing.T) {
	m := &audit.Memory{}
	err := audit.Record(context.Background(), m, audit.Event{
		Actor:    "admin",
		Action:   "user.delete",
		Resource: "user:1",
	})
	if err != nil {
		t.Fatal(err)
	}
	ev := m.Snapshot()
	if len(ev) != 1 || ev[0].Action != "user.delete" {
		t.Fatalf("%+v", ev)
	}
}

func TestMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := &audit.Memory{}
	r := gin.New()
	r.Use(audit.Middleware(m))
	r.GET("/admin/users", func(c *gin.Context) {
		auth.SetUser(c, auth.User{ID: "u42", Email: "a@example.com"})
		c.Status(http.StatusOK)
	})
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/users", nil))
	ev := m.Snapshot()
	if len(ev) != 1 {
		t.Fatalf("events=%d", len(ev))
	}
	if ev[0].Actor != "u42" {
		t.Fatalf("actor=%q", ev[0].Actor)
	}
	if ev[0].Meta["method"] != http.MethodGet || ev[0].Meta["path"] != "/admin/users" {
		t.Fatalf("meta=%v", ev[0].Meta)
	}
	if ev[0].Meta["status"] != http.StatusOK {
		t.Fatalf("status=%v", ev[0].Meta["status"])
	}

	// health skipped by default
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if len(m.Snapshot()) != 1 {
		t.Fatalf("health should be skipped, events=%d", len(m.Snapshot()))
	}
}
