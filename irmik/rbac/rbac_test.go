package rbac_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/auth"
	"github.com/boracomet/go-irmik/irmik/rbac"
)

func TestRegistryPermissions(t *testing.T) {
	reg := rbac.New()
	reg.GrantRolePermissions("editor", "posts:write", "posts:read")
	reg.GrantRolePermissions("admin", "posts:write", "posts:read", "users:manage")
	reg.AssignRoles("u1", "editor")

	u := auth.User{ID: "u1"}
	if !reg.HasPermission(u, "posts:write") {
		t.Fatal("editor should write")
	}
	if reg.HasPermission(u, "users:manage") {
		t.Fatal("editor should not manage users")
	}
	if !reg.HasRole(u, "editor") {
		t.Fatal("missing role")
	}

	admin := auth.User{ID: "a1", Roles: []string{"admin"}}
	if !reg.HasPermission(admin, "users:manage") {
		t.Fatal("admin roles on user struct")
	}
}

func TestRequirePermissionMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reg := rbac.New()
	reg.GrantRolePermissions("admin", "secret:read")

	r := gin.New()
	r.GET("/secret", func(c *gin.Context) {
		auth.SetUser(c, auth.User{ID: "1", Roles: []string{"admin"}})
		c.Next()
	}, rbac.RequirePermission(reg, "secret:read"), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	r.GET("/denied", func(c *gin.Context) {
		auth.SetUser(c, auth.User{ID: "2", Roles: []string{"guest"}})
		c.Next()
	}, rbac.RequirePermission(reg, "secret:read"), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/secret", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("got %d", w.Code)
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/denied", nil))
	if w2.Code != http.StatusForbidden {
		t.Fatalf("got %d", w2.Code)
	}
}
