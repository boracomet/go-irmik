package validate_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/validate"
)

type loginReq struct {
	Email    string `json:"email" form:"email" validate:"required,email"`
	Password string `json:"password" form:"password" validate:"required,min=8"`
}

func TestStructOK(t *testing.T) {
	err := validate.Struct(loginReq{Email: "a@b.co", Password: "secret123"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestStructErrors(t *testing.T) {
	err := validate.Struct(loginReq{Email: "bad", Password: "short"})
	ve, ok := validate.AsErrors(err)
	if !ok {
		t.Fatalf("expected Errors, got %T %v", err, err)
	}
	if _, ok := ve["Email"]; !ok {
		t.Fatalf("missing Email: %#v", ve)
	}
	if _, ok := ve["Password"]; !ok {
		t.Fatalf("missing Password: %#v", ve)
	}
}

func TestBindJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/", func(c *gin.Context) {
		var req loginReq
		if err := validate.BindJSON(c, &req); err != nil {
			validate.Abort(c, err)
			return
		}
		c.JSON(http.StatusOK, req)
	})

	w := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"email":"u@example.com","password":"password1"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestBindJSONInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/", func(c *gin.Context) {
		var req loginReq
		if err := validate.BindJSON(c, &req); err != nil {
			validate.Abort(c, err)
			return
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"email":"nope","password":"x"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
