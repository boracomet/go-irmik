package island

import (
	"encoding/json"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boracomet/go-irmik/irmik/config"
)

func TestParseManifestLookup(t *testing.T) {
	raw := `{
		"islands/Counter.tsx": {
			"file": "assets/Counter-abc.js",
			"src": "islands/Counter.tsx",
			"isEntry": true,
			"css": ["assets/Counter-abc.css"],
			"imports": ["_vendor.js"]
		},
		"_vendor.js": {
			"file": "assets/vendor-xyz.js",
			"css": ["assets/vendor.css"]
		}
	}`
	m, err := ParseManifest([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	chunk, key, ok := m.Lookup("Counter")
	if !ok || key != "islands/Counter.tsx" {
		t.Fatalf("Lookup Counter: ok=%v key=%q", ok, key)
	}
	if chunk.File != "assets/Counter-abc.js" {
		t.Fatalf("file = %q", chunk.File)
	}
	js, css, err := m.EntryAssets("Counter")
	if err != nil {
		t.Fatal(err)
	}
	if js != "assets/Counter-abc.js" {
		t.Fatalf("js = %q", js)
	}
	if len(css) != 2 {
		t.Fatalf("css = %#v", css)
	}
}

func TestLoadManifestFromOutDir(t *testing.T) {
	dir := t.TempDir()
	viteDir := filepath.Join(dir, ".vite")
	if err := os.MkdirAll(viteDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"islands/A.tsx":{"file":"assets/A.js","isEntry":true}}`
	path := filepath.Join(viteDir, "manifest.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m, found, err := LoadManifestFromOutDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if found != path {
		t.Fatalf("found = %q want %q", found, path)
	}
	if _, _, ok := m.Lookup("A"); !ok {
		t.Fatal("expected Lookup A")
	}
}

func TestRenderDev(t *testing.T) {
	mgr, err := New(Options{
		Enabled:   true,
		Dev:       true,
		DevServer: "http://localhost:5173",
		Dir:       "islands",
	})
	if err != nil {
		t.Fatal(err)
	}
	html, err := mgr.Render("Counter", map[string]any{"initial": 3})
	if err != nil {
		t.Fatal(err)
	}
	s := string(html)
	if !strings.Contains(s, `data-island="Counter"`) {
		t.Fatalf("missing mount: %s", s)
	}
	if !strings.Contains(s, `data-props='`) || !strings.Contains(s, `initial`) || !strings.Contains(s, `:3`) {
		t.Fatalf("missing props: %s", s)
	}
	if !strings.Contains(s, `src="http://localhost:5173/islands/Counter.tsx"`) {
		t.Fatalf("missing dev script: %s", s)
	}
	rt := string(mgr.Runtime())
	if !strings.Contains(rt, "/@vite/client") {
		t.Fatalf("runtime = %s", rt)
	}
}

func TestRenderProd(t *testing.T) {
	dir := t.TempDir()
	manifest := Manifest{
		"islands/Counter.tsx": {
			File:    "assets/Counter-hash.js",
			IsEntry: true,
			CSS:     []string{"assets/Counter-hash.css"},
		},
	}
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	mgr, err := New(Options{
		Enabled:    true,
		Dev:        false,
		OutDir:     dir,
		PublicPath: "islands",
	})
	if err != nil {
		t.Fatal(err)
	}
	html, err := mgr.Render("Counter", nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(html)
	if !strings.Contains(s, `data-props='{}'`) {
		t.Fatalf("props: %s", s)
	}
	if !strings.Contains(s, `href="/islands/assets/Counter-hash.css"`) {
		t.Fatalf("css: %s", s)
	}
	if !strings.Contains(s, `src="/islands/assets/Counter-hash.js"`) {
		t.Fatalf("js: %s", s)
	}
	if string(mgr.Runtime()) != "" {
		t.Fatal("prod runtime should be empty")
	}
}

func TestFuncMapIntegration(t *testing.T) {
	mgr, err := New(Options{Enabled: true, Dev: true, DevServer: "http://localhost:5173"})
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := template.New("t").Funcs(mgr.FuncMap()).Parse(`{{ island "Counter" . }}`)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := tpl.Execute(&b, map[string]int{"n": 1}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `data-island="Counter"`) {
		t.Fatalf("got %s", b.String())
	}
}

func TestFromConfigDisabled(t *testing.T) {
	mgr, err := FromConfig(config.IslandsConfig{Enabled: false}, true)
	if err != nil {
		t.Fatal(err)
	}
	html, err := mgr.Render("Counter", nil)
	if err != nil || html != "" {
		t.Fatalf("got %q err=%v", html, err)
	}
}

func TestValidateName(t *testing.T) {
	mgr, _ := New(Options{Enabled: true, Dev: true})
	if _, err := mgr.Render("../evil", nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestEncodePropsEscapes(t *testing.T) {
	s, err := encodeProps(map[string]string{"x": `a'b"c`})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(s, `'`) && !strings.Contains(s, "&#39;") {
		// raw single quote must not appear unescaped in attribute body
		t.Fatalf("unescaped quote in %q", s)
	}
}
