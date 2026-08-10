package tmplfunc

import (
	"html/template"
	"strings"
	"testing"
	"time"
)

func TestDictAndSet(t *testing.T) {
	m, err := Dict("a", 1, "b", "x")
	if err != nil || m["a"] != 1 || m["b"] != "x" {
		t.Fatalf("Dict = %#v err=%v", m, err)
	}
	if _, err := Dict("only"); err == nil {
		t.Fatal("expected odd-args error")
	}
	Set(m, "c", true)
	if m["c"] != true {
		t.Fatal("Set failed")
	}
}

func TestArithmeticAndUntil(t *testing.T) {
	if Add(2, 3) != 5 || Sub(5, 2) != 3 || Div(10, 4) != 2 || Mod(10, 4) != 2 {
		t.Fatal("arithmetic")
	}
	if Div(1, 0) != 0 || Mod(1, 0) != 0 {
		t.Fatal("div/mod zero")
	}
	u := Until(3)
	if len(u) != 3 || u[2] != 2 {
		t.Fatalf("Until = %#v", u)
	}
}

func TestFormatDateTR(t *testing.T) {
	ts := time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC)
	got := FormatDateTime(ts, "tr")
	if got != "25 Aralık 2025" {
		t.Fatalf("got %q", got)
	}
	got = FormatDate(ts.Format(time.RFC3339), "en")
	if !strings.Contains(got, "December") {
		t.Fatalf("got %q", got)
	}
}

func TestMapExecutes(t *testing.T) {
	tpl, err := template.New("t").Funcs(Map()).Parse(`{{slugify "Hello World"}}|{{add 1 2}}`)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := tpl.Execute(&b, nil); err != nil {
		t.Fatal(err)
	}
	if b.String() != "hello-world|3" {
		t.Fatalf("got %q", b.String())
	}
}
