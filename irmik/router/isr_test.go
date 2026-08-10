package router_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/cache"
	"github.com/boracomet/go-irmik/irmik/meta"
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
