package render_test

import (
	"os"
	"path/filepath"
	"strings"
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
