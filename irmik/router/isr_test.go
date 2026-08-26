package router_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/cache"
	"github.com/boracomet/go-irmik/irmik/meta"
	"github.com/boracomet/go-irmik/irmik/middleware"
	"github.com/boracomet/go-irmik/irmik/render"
	"github.com/boracomet/go-irmik/irmik/router"
)

func TestISREtagAndXCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	page := filepath.Join(dir, "page.html")
	if err := os.WriteFile(page, []byte(`<p>hello-isr</p>`), 0o644); err != nil {
		t.Fatal(err)
	}
	metaPath := filepath.Join(dir, "_meta.yaml")
	if err := os.WriteFile(metaPath, []byte("mode: isr\nrevalidate: 1h\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := render.New(render.Options{AppDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	store := cache.NewMemory()
	rt, err := router.New(router.Options{
		AppDir:   dir,
		Locale:   "en",
		Cache:    store,
		Renderer: eng,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.Routes()) != 1 || rt.Routes()[0].Meta.Mode != meta.ModeISR {
		t.Fatalf("unexpected routes: %+v", rt.Routes())
	}

	e := gin.New()
	rt.Mount(e)

	// MISS
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	e.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("miss status %d", w.Code)
	}
	if got := w.Header().Get("X-Cache"); got != "MISS" {
		t.Fatalf("X-Cache want MISS got %q", got)
	}
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatal("missing ETag")
	}

	// HIT
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	e.ServeHTTP(w2, req2)
	if got := w2.Header().Get("X-Cache"); got != "HIT" {
		t.Fatalf("X-Cache want HIT got %q", got)
	}

	// 304
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.Header.Set("If-None-Match", etag)
	e.ServeHTTP(w3, req3)
	if w3.Code != http.StatusNotModified {
		t.Fatalf("want 304 got %d", w3.Code)
	}
}

func TestISRHEADSharesGETCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "page.html"), []byte(`<p>head-share</p>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "_meta.yaml"), []byte("mode: isr\nrevalidate: 1h\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := render.New(render.Options{AppDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	rt, err := router.New(router.Options{
		AppDir: dir, Locale: "en", Cache: cache.NewMemory(), Renderer: eng,
	})
	if err != nil {
		t.Fatal(err)
	}
	e := gin.New()
	rt.Mount(e)

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("GET X-Cache=%q", w.Header().Get("X-Cache"))
	}
	etag := w.Header().Get("ETag")

	wh := httptest.NewRecorder()
	e.ServeHTTP(wh, httptest.NewRequest(http.MethodHead, "/", nil))
	if wh.Code != 200 {
		t.Fatalf("HEAD status %d", wh.Code)
	}
	if got := wh.Header().Get("X-Cache"); got != "HIT" {
		t.Fatalf("HEAD should share GET cache, X-Cache=%q", got)
	}
	if wh.Header().Get("ETag") != etag {
		t.Fatalf("HEAD ETag %q GET ETag %q", wh.Header().Get("ETag"), etag)
	}
	if wh.Body.Len() != 0 {
		t.Fatalf("HEAD body should be empty, got %q", wh.Body.String())
	}
}

func TestISRRevalidateRunsLoader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "page.html"), []byte(`n={{.Data.n}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "_meta.yaml"), []byte("mode: isr\nrevalidate: 30ms\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := render.New(render.Options{AppDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	var n atomic.Int32
	rt, err := router.New(router.Options{
		AppDir:   dir,
		Locale:   "en",
		Cache:    cache.NewMemory(),
		Renderer: eng,
		Loaders: map[string]router.Loader{
			"/": func(c *gin.Context) (any, error) {
				return map[string]any{"n": n.Add(1)}, nil
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	e := gin.New()
	rt.Mount(e)

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(w.Body.String(), "n=1") {
		t.Fatalf("first render: %q", w.Body.String())
	}

	time.Sleep(40 * time.Millisecond)
	w2 := httptest.NewRecorder()
	e.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := w2.Header().Get("X-Cache"); got != "STALE" && got != "HIT" {
		t.Fatalf("expected STALE or HIT after TTL, got %q body=%q", got, w2.Body.String())
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		w3 := httptest.NewRecorder()
		e.ServeHTTP(w3, httptest.NewRequest(http.MethodGet, "/", nil))
		if strings.Contains(w3.Body.String(), "n=2") {
			if n.Load() < 2 {
				t.Fatalf("body has n=2 but loader count=%d", n.Load())
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("ISR revalidate did not re-run the loader")
}

func TestSSRRenderErrorIsGeneric(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "page.html"), []byte(`ok`), 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := render.New(render.Options{AppDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	rt, err := router.New(router.Options{
		AppDir:   dir,
		Renderer: eng,
		Loaders: map[string]router.Loader{
			"/": func(c *gin.Context) (any, error) {
				return nil, os.ErrPermission
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	e := gin.New()
	e.Use(middleware.RequestID())
	rt.Mount(e)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "req-test-1")
	e.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "permission") || strings.Contains(body, "render error") {
		t.Fatalf("client body leaked internal error: %q", body)
	}
	if body != "internal server error" {
		t.Fatalf("body=%q", body)
	}
}
