package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvAndFileMulti(t *testing.T) {
	t.Setenv("IRMIK_TEST_SECRET", "from-env")
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if err := os.WriteFile(path, []byte(" from-file \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := Multi{Providers: []Provider{
		Env{Prefix: "IRMIK_"},
		Files{"OTHER": path},
	}}
	v, err := p.Get("TEST_SECRET")
	if err != nil || v != "from-env" {
		t.Fatalf("env: %v %q", err, v)
	}
	v, err = p.Get("OTHER")
	// Env fails, Files succeeds via Multi — but Multi tries Env first for OTHER too.
	// Env{Prefix} looks up IRMIK_OTHER which is unset, then Files.
	if err != nil || v != "from-file" {
		t.Fatalf("file: %v %q", err, v)
	}
}

func TestCached(t *testing.T) {
	inner := Map{"k": "v"}
	c := &Cached{Inner: inner}
	v, err := c.Get("k")
	if err != nil || v != "v" {
		t.Fatal(err, v)
	}
	delete(inner, "k")
	v, err = c.Get("k")
	if err != nil || v != "v" {
		t.Fatalf("cache miss after delete: %v %q", err, v)
	}
}
