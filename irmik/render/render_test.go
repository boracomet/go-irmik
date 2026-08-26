package render_test

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/boracomet/go-irmik/irmik/render"
)

func TestTmplfuncMergedAndIslandDefault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	page := filepath.Join(dir, "page.html")
	if err := os.WriteFile(page, []byte(`{{ with dict "n" 1 }}{{ .n }}{{ end }}|{{ island "Counter" }}`), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := render.New(render.Options{AppDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	out, err := eng.RenderToBytes(page, nil, render.Data{})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "1|") {
		t.Fatalf("expected dict helper output, got %q", s)
	}
	if !strings.Contains(s, `data-island="Counter"`) {
		t.Fatalf("expected island markup, got %q", s)
	}
}

func TestCompiledCacheIgnoresDiskUntilReload(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	page := filepath.Join(dir, "page.html")
	layout := filepath.Join(dir, "layout.html")
	if err := os.WriteFile(page, []byte("page-v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(layout, []byte(`<wrap>{{.Content}}</wrap>`), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := render.New(render.Options{AppDir: dir})
	if err != nil {
		t.Fatal(err)
	}

	out, err := eng.RenderToBytes(page, []string{layout}, render.Data{})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); got != "<wrap>page-v1</wrap>" {
		t.Fatalf("first render: got %q", got)
	}

	if err := os.WriteFile(page, []byte("page-v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(layout, []byte(`<new>{{.Content}}</new>`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = eng.RenderToBytes(page, []string{layout}, render.Data{})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); got != "<wrap>page-v1</wrap>" {
		t.Fatalf("cached render should ignore disk edits, got %q", got)
	}

	if err := os.Remove(page); err != nil {
		t.Fatal(err)
	}
	out, err = eng.RenderToBytes(page, []string{layout}, render.Data{})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); got != "<wrap>page-v1</wrap>" {
		t.Fatalf("cached render should ignore missing file, got %q", got)
	}

	if err := eng.Reload(); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.RenderToBytes(page, []string{layout}, render.Data{}); err == nil {
		t.Fatal("expected read error after Reload with missing page")
	}

	if err := os.WriteFile(page, []byte("page-v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = eng.RenderToBytes(page, []string{layout}, render.Data{})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); got != "<new>page-v2</new>" {
		t.Fatalf("after Reload expected disk contents, got %q", got)
	}
}

func TestDefineIsolationAcrossPages(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pageA := filepath.Join(dir, "a.html")
	pageB := filepath.Join(dir, "b.html")
	if err := os.WriteFile(pageA, []byte(`{{define "box"}}A{{end}}{{template "box"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pageB, []byte(`{{define "box"}}B{{end}}{{template "box"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := render.New(render.Options{AppDir: dir})
	if err != nil {
		t.Fatal(err)
	}

	outA, err := eng.RenderToBytes(pageA, nil, render.Data{})
	if err != nil {
		t.Fatal(err)
	}
	outB, err := eng.RenderToBytes(pageB, nil, render.Data{})
	if err != nil {
		t.Fatal(err)
	}
	if string(outA) != "A" {
		t.Fatalf("page A: got %q", outA)
	}
	if string(outB) != "B" {
		t.Fatalf("page B: got %q", outB)
	}

	// Re-render in reverse order from cache; defines must not leak.
	outA, err = eng.RenderToBytes(pageA, nil, render.Data{})
	if err != nil {
		t.Fatal(err)
	}
	if string(outA) != "A" {
		t.Fatalf("cached page A: got %q", outA)
	}
}

func TestSetFuncsInvalidatesCompiledCache(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	page := filepath.Join(dir, "page.html")
	if err := os.WriteFile(page, []byte(`{{ who }}`), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := render.New(render.Options{
		AppDir: dir,
		Funcs:  template.FuncMap{"who": func() string { return "first" }},
	})
	if err != nil {
		t.Fatal(err)
	}

	out, err := eng.RenderToBytes(page, nil, render.Data{})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); got != "first" {
		t.Fatalf("got %q", got)
	}

	eng.SetFuncs(template.FuncMap{"who": func() string { return "second" }})
	out, err = eng.RenderToBytes(page, nil, render.Data{})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); got != "second" {
		t.Fatalf("SetFuncs should drop compiled templates, got %q", got)
	}
}

func TestSetIslandFuncInvalidatesCompiledCache(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	page := filepath.Join(dir, "page.html")
	if err := os.WriteFile(page, []byte(`{{ island "X" }}`), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := render.New(render.Options{AppDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	out, err := eng.RenderToBytes(page, nil, render.Data{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `data-island="X"`) {
		t.Fatalf("default island: got %q", out)
	}

	eng.SetIslandFunc(func(name string, _ any) (template.HTML, error) {
		return template.HTML("ISLAND:" + name), nil
	})
	out, err = eng.RenderToBytes(page, nil, render.Data{})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); got != "ISLAND:X" {
		t.Fatalf("SetIslandFunc should be visible on next render, got %q", got)
	}
}

func TestConcurrentCachedRenders(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	page := filepath.Join(dir, "page.html")
	layout := filepath.Join(dir, "layout.html")
	if err := os.WriteFile(page, []byte(`{{ .Path }}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(layout, []byte(`{{.Content}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := render.New(render.Options{AppDir: dir})
	if err != nil {
		t.Fatal(err)
	}

	const n = 32
	var wg sync.WaitGroup
	wg.Add(n)
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			out, err := eng.RenderToBytes(page, []string{layout}, render.Data{Path: "ok"})
			if err != nil {
				errCh <- err
				return
			}
			if string(out) != "ok" {
				errCh <- fmt.Errorf("got %q", out)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
}

func TestPartialsStayOnCompiledPages(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	partials := filepath.Join(dir, "partials")
	if err := os.MkdirAll(partials, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(partials, "head.html"), []byte("HEAD"), 0o644); err != nil {
		t.Fatal(err)
	}
	page := filepath.Join(dir, "page.html")
	if err := os.WriteFile(page, []byte(`{{template "head.html"}}-page`), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, err := render.New(render.Options{AppDir: dir, TemplatesDir: partials})
	if err != nil {
		t.Fatal(err)
	}
	out, err := eng.RenderToBytes(page, nil, render.Data{})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); got != "HEAD-page" {
		t.Fatalf("first render: got %q", got)
	}

	if err := os.WriteFile(filepath.Join(partials, "head.html"), []byte("NEW"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = eng.RenderToBytes(page, nil, render.Data{})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); got != "HEAD-page" {
		t.Fatalf("cached render should keep compiled partial, got %q", got)
	}

	if err := eng.Reload(); err != nil {
		t.Fatal(err)
	}
	out, err = eng.RenderToBytes(page, nil, render.Data{})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); got != "NEW-page" {
		t.Fatalf("after Reload expected new partial, got %q", got)
	}
}
