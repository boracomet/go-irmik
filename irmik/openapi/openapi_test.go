package openapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDocServe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	d := New("Demo", "1.0.0")
	d.Add("/users", http.MethodGet, Operation{Summary: "List users", Tags: []string{"users"}})

	r := gin.New()
	d.Mount(r, "/openapi.json")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["openapi"] != "3.0.3" {
		t.Fatalf("openapi=%v", out["openapi"])
	}
	paths, _ := out["paths"].(map[string]any)
	if paths["/users"] == nil {
		t.Fatal("missing /users")
	}
}
