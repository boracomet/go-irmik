package devtools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func testEngine(t *testing.T) (*gin.Engine, *Dev) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	d := New(Options{Info: func() Snapshot {
		return Snapshot{Env: "development", Addr: "127.0.0.1:8080", Routes: []RouteInfo{{Path: "/", Mode: "ssr"}}}
	}})
	r.Use(d.Inject())
	d.Mount(r)
	r.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte("<html><body><h1>hi</h1></body></html>"))
	})
	r.GET("/api", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/boom", func(c *gin.Context) {
		c.String(http.StatusInternalServerError, "render error: broken")
	})
	return r, d
}

func TestInjectHTML(t *testing.T) {
	r, _ := testEngine(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "/_irmik/dev/overlay.js") {
		t.Fatalf("missing overlay: %s", body)
	}
	if !strings.Contains(body, "</body>") {
		t.Fatal("body tag lost")
	}
}

func TestSkipJSON(t *testing.T) {
	r, _ := testEngine(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	r.ServeHTTP(w, req)
	if strings.Contains(w.Body.String(), "overlay.js") {
		t.Fatal("injected JSON")
	}
}

func TestWrapPlainError(t *testing.T) {
	r, _ := testEngine(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	req.Header.Set("Accept", "text/html")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "render error: broken") {
		t.Fatalf("missing error: %s", body)
	}
	if !strings.Contains(body, "overlay.js") {
		t.Fatal("500 should still get overlay")
	}
}

func TestLogoAndInfo(t *testing.T) {
	r, _ := testEngine(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/_irmik/dev/logo.png", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("Content-Type"), "image/png") {
		t.Fatalf("logo status=%d ct=%s", w.Code, w.Header().Get("Content-Type"))
	}
	if w.Body.Len() < 100 {
		t.Fatal("empty logo")
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/_irmik/dev/info", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("info %d", w.Code)
	}
	var snap Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Addr != "127.0.0.1:8080" || len(snap.Routes) != 1 {
		t.Fatalf("%+v", snap)
	}
}

func TestHubBroadcast(t *testing.T) {
	d := New(Options{})
	ch := d.subscribe()
	d.Reload("app/page.html")
	ev := <-ch
	if ev.name != "reload" {
		t.Fatalf("name=%s", ev.name)
	}
	d.unsubscribe(ch)
	d.Report("template", "parse error")
}

func TestOverlayJSServed(t *testing.T) {
	r, _ := testEngine(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/_irmik/dev/overlay.js", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Irmik Dev") {
		t.Fatalf("js status=%d len=%d", w.Code, w.Body.Len())
	}
}
