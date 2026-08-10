package htmx_test

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/htmx"
)

func TestIsRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var got bool
	r.GET("/", func(c *gin.Context) {
		got = htmx.IsRequest(c)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)
	if got {
		t.Fatal("expected false without HX-Request")
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(htmx.HeaderRequest, "true")
	r.ServeHTTP(w, req)
	if !got {
		t.Fatal("expected true")
	}
}

func TestHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/save", func(c *gin.Context) {
		htmx.Redirect(c, "/users")
		htmx.Trigger(c, "saved")
		htmx.Retarget(c, "#main")
		htmx.Reswap(c, "outerHTML")
		htmx.PushURL(c, "/users/1")
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/save", nil))
	h := w.Header()
	if h.Get(htmx.HeaderRedirect) != "/users" {
		t.Fatalf("redirect=%q", h.Get(htmx.HeaderRedirect))
	}
	if h.Get(htmx.HeaderTrigger) != "saved" {
		t.Fatalf("trigger=%q", h.Get(htmx.HeaderTrigger))
	}
	if h.Get(htmx.HeaderRetarget) != "#main" {
		t.Fatalf("retarget=%q", h.Get(htmx.HeaderRetarget))
	}
	if h.Get(htmx.HeaderReswap) != "outerHTML" {
		t.Fatalf("reswap=%q", h.Get(htmx.HeaderReswap))
	}
	if h.Get(htmx.HeaderPushURL) != "/users/1" {
		t.Fatalf("push=%q", h.Get(htmx.HeaderPushURL))
	}
}

func TestRenderPartial(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tmpl := template.Must(template.New("row").Parse(`<tr id="u">{{.Name}}</tr>`))
	r := gin.New()
	r.GET("/partial", func(c *gin.Context) {
		if err := htmx.RenderPartial(c, tmpl, "row", map[string]any{"Name": "Ada"}); err != nil {
			t.Errorf("render: %v", err)
		}
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/partial", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("content-type=%q", ct)
	}
	if body := w.Body.String(); body != `<tr id="u">Ada</tr>` {
		t.Fatalf("body=%q", body)
	}
}
