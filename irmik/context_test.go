package irmik

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/auth"
)

func TestMustUserPanicsWithoutUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	ic := FromGin(c)
	defer func() {
		if recover() == nil {
			t.Fatal("MustUser must panic when no user is in context")
		}
	}()
	_ = ic.MustUser()
}

func TestMustUserReturnsAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	auth.SetUser(c, auth.User{ID: "u1", Email: "a@example.com"})
	got := FromGin(c).MustUser()
	if got.ID != "u1" || got.Email != "a@example.com" {
		t.Fatalf("MustUser = %+v", got)
	}
}
