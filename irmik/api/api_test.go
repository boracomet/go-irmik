package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/api"
	"github.com/boracomet/go-irmik/irmik/validate"
)

func TestErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/err", func(c *gin.Context) {
		api.Error(c, http.StatusNotFound, "not_found", "item missing", gin.H{"id": "9"})
	})
	r.GET("/abort", func(c *gin.Context) {
		api.Abort(c, http.StatusBadRequest, "bad_request", "nope")
		return
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/err", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d", w.Code)
	}
	var env api.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Code != "not_found" || env.Error.Message != "item missing" {
		t.Fatalf("envelope=%+v", env)
	}
	details, ok := env.Error.Details.(map[string]any)
	if !ok || details["id"] != "9" {
		t.Fatalf("details=%v", env.Error.Details)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/abort", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("abort status=%d", w.Code)
	}
	if strings.Contains(w.Body.String(), "should") {
		t.Fatalf("expected abort to stop chain, body=%s", w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Code != "bad_request" {
		t.Fatalf("code=%q", env.Error.Code)
	}
}

func TestJSONAndV1(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := api.V1(r)
	v1.GET("/ping", func(c *gin.Context) {
		api.JSON(c, http.StatusOK, gin.H{"ok": true})
	})
	api.MountV1(r, func(g *gin.RouterGroup) {
		g.GET("/mounted", func(c *gin.Context) {
			api.JSON(c, 0, gin.H{"mounted": true})
		})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatalf("ping: %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/mounted", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"mounted":true`) {
		t.Fatalf("mounted: %d %s", w.Code, w.Body.String())
	}
}

func TestAbortValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	type body struct {
		Email string `json:"email" validate:"required,email"`
	}
	r := gin.New()
	r.POST("/x", func(c *gin.Context) {
		var b body
		if err := api.BindJSON(c, &b); err != nil {
			api.AbortValidation(c, err)
			return
		}
		api.JSON(c, http.StatusOK, b)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"email":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var env api.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Code != "validation_failed" {
		t.Fatalf("code=%q", env.Error.Code)
	}
	if _, ok := validate.AsErrors(validate.Errors{"Email": {"must be a valid email"}}); !ok {
		t.Fatal("sanity")
	}
	if env.Error.Details == nil {
		t.Fatal("expected field details")
	}
}

func TestHelpers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/404", func(c *gin.Context) { api.NotFound(c, "") })
	r.GET("/401", func(c *gin.Context) { api.Unauthorized(c, "") })
	r.GET("/403", func(c *gin.Context) { api.Forbidden(c, "") })
	r.GET("/500", func(c *gin.Context) { api.Internal(c, "") })

	cases := []struct {
		path string
		code int
		err  string
	}{
		{"/404", 404, "not_found"},
		{"/401", 401, "unauthorized"},
		{"/403", 403, "forbidden"},
		{"/500", 500, "internal_error"},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if w.Code != tc.code {
			t.Fatalf("%s status=%d", tc.path, w.Code)
		}
		var env api.ErrorResponse
		_ = json.Unmarshal(w.Body.Bytes(), &env)
		if env.Error.Code != tc.err {
			t.Fatalf("%s code=%q", tc.path, env.Error.Code)
		}
	}
}
