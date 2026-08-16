package imagex

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPipelineHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hero.png"), testPNG(t, 80, 40), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &Pipeline{Root: dir, Widths: []int{375, 1440}}
	r := gin.New()
	if err := p.Mount(r); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/_irmik/img?src=/hero.png&w=375", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "image/webp" {
		t.Fatalf("ct=%s", rec.Header().Get("Content-Type"))
	}
	if rec.Body.Len() == 0 {
		t.Fatal("empty body")
	}
}

func TestPipelineRejects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	p := &Pipeline{Root: dir}
	r := gin.New()
	if err := p.Mount(r); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		url  string
		code int
	}{
		{"dotdot", "/_irmik/img?src=../secret.png&w=375", http.StatusBadRequest},
		{"remote", "/_irmik/img?src=https://evil.example/x.jpg&w=375", http.StatusBadRequest},
		{"width", "/_irmik/img?src=/hero.png&w=12", http.StatusBadRequest},
		{"missing", "/_irmik/img?src=/nope.png&w=375", http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			r.ServeHTTP(rec, req)
			if rec.Code != tc.code {
				t.Fatalf("status=%d want %d", rec.Code, tc.code)
			}
		})
	}
}

func TestPipelineRootRequired(t *testing.T) {
	p := &Pipeline{}
	if _, err := p.Handler(); err == nil {
		t.Fatal("expected error")
	}
}

func TestImgHelper(t *testing.T) {
	p := &Pipeline{Root: t.TempDir()}
	html, err := p.Img("/hero.jpg", map[string]any{
		"alt":      `cat "yarn"`,
		"width":    1440,
		"height":   810,
		"priority": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(html)
	for _, want := range []string{
		`srcset="`,
		`w=375 375w`,
		`w=1440 1440w`,
		`alt="cat &#34;yarn&#34;"`,
		`width="1440"`,
		`height="810"`,
		`fetchpriority="high"`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %s", want, s)
		}
	}
	if strings.Contains(s, `loading="lazy"`) {
		t.Fatal("priority image must not be lazy")
	}

	lazy, err := p.Img("/hero.jpg", "plain")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(lazy), `loading="lazy"`) {
		t.Fatal("non-priority should lazy-load")
	}
	if !strings.Contains(string(lazy), `alt="plain"`) {
		t.Fatal("alt")
	}
}

func TestImgRejectsRemote(t *testing.T) {
	p := &Pipeline{Root: t.TempDir()}
	if _, err := p.Img("https://evil.example/x.jpg"); err == nil {
		t.Fatal("expected error")
	}
}

func TestVariantsAndWrite(t *testing.T) {
	src := bytes.NewReader(testPNG(t, 200, 100))
	vs, err := Variants(src, []int{50, 100}, Options{Format: PNG})
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 2 {
		t.Fatalf("len=%d", len(vs))
	}
	decoded, err := png.Decode(bytes.NewReader(vs[0].Bytes))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != 50 {
		t.Fatalf("w=%d", decoded.Bounds().Dx())
	}

	dir := t.TempDir()
	paths, err := WriteVariants(dir, "photo.jpg", bytes.NewReader(testPNG(t, 80, 40)), []int{375}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	p375, ok := paths[375]
	if !ok || filepath.Base(p375) != "photo-375.webp" {
		t.Fatalf("paths=%v", paths)
	}
	if _, err := os.Stat(p375); err != nil {
		t.Fatal(err)
	}
}
