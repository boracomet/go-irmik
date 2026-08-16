package compress

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGzip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Gzip())
	r.GET("/", func(c *gin.Context) {
		c.String(200, "hello compress")
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	r.ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("encoding=%q", rec.Header().Get("Content-Encoding"))
	}
	gr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = gr.Close() }()
	data, err := io.ReadAll(gr)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello compress" {
		t.Fatalf("body=%q", data)
	}
}
