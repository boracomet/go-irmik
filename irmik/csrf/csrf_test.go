package csrf_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/csrf"
	"github.com/boracomet/go-irmik/irmik/session"
)

func TestCSRFRejectsMissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr, err := session.NewManager(session.Options{
		Name: "s", MaxAge: time.Hour, Path: "/", HTTPOnly: true, Store: session.NewMemory(),
	})
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.Use(mgr.Middleware(), csrf.Middleware(csrf.Options{}))
	r.GET("/t", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"csrf": csrf.Token(c)})
	})
	r.POST("/t", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest(http.MethodGet, "/t", nil))
	if w1.Code != http.StatusOK {
		t.Fatalf("get: %d", w1.Code)
	}
	cookies := w1.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected cookie")
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/t", nil)
	req2.AddCookie(cookies[0])
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", w2.Code)
	}
}

func TestGenerateValid(t *testing.T) {
	tok, err := csrf.Generate()
	if err != nil || tok == "" {
		t.Fatal(err)
	}
	if !csrf.Valid(tok, tok) {
		t.Fatal("should match")
	}
	if csrf.Valid(tok, tok+"x") {
		t.Fatal("should not match")
	}
}
