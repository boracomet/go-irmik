package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewScaffoldGoModHasNoReplace(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	cmd := Root()
	cmd.SetArgs([]string{"new", "hello"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	modPath := filepath.Join("hello", "go.mod")
	b, err := os.ReadFile(modPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if strings.Contains(got, "replace ") {
		t.Fatalf("scaffold go.mod must not contain a sibling replace:\n%s", got)
	}
	if strings.Contains(got, "v0.0.0") {
		t.Fatalf("scaffold go.mod must pin a real version, not v0.0.0:\n%s", got)
	}
	want := "require github.com/boracomet/go-irmik " + ScaffoldModuleVersion
	if !strings.Contains(got, want) {
		t.Fatalf("go.mod missing %q:\n%s", want, got)
	}

	mainGo, err := os.ReadFile(filepath.Join("hello", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(mainGo, []byte(`GET("/health"`)) {
		t.Fatal("scaffold must not double-register GET /health (irmik.New already mounts it)")
	}
}

func TestScaffoldGoModHelper(t *testing.T) {
	got := scaffoldGoMod("demo")
	if strings.Contains(got, "replace ") {
		t.Fatalf("replace present: %s", got)
	}
	if !strings.HasPrefix(got, "module demo\n") {
		t.Fatalf("module line: %q", got)
	}
}
