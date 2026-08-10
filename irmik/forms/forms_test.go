package forms

import (
	"strings"
	"testing"
)

func TestCSRFInputEscapes(t *testing.T) {
	html := string(CSRFInput(`"><script>alert(1)</script>`))
	if strings.Contains(html, "<script>") {
		t.Fatalf("script not escaped: %s", html)
	}
	if !strings.Contains(html, `type="hidden"`) || !strings.Contains(html, `name="_csrf"`) {
		t.Fatalf("unexpected html: %s", html)
	}
}
