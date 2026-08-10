package rbac_test

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/auth"
	"github.com/boracomet/go-irmik/irmik/rbac"
)

func TestPerm(t *testing.T) {
	if got := rbac.Perm("posts", "read"); got != "posts:read" {
		t.Fatalf("got %q", got)
	}
}

func TestRegistryPermissions(t *testing.T) {
	reg := rbac.New()
	reg.Grant("editor", "posts:write", "posts:read")
	reg.Grant("admin", "posts:write", "posts:read", "users:manage")
	reg.AssignRoles("u1", "editor")

	u := auth.User{ID: "u1"}
	if !reg.Has(u, "posts:write") {
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

	perms := reg.PermissionsFor("editor")
	if len(perms) != 2 {
		t.Fatalf("PermissionsFor: %v", perms)
	}
}

func TestApplyPresets(t *testing.T) {
	reg := rbac.New()
	reg.ApplyPresets("items")
	admin := auth.User{Roles: []string{rbac.RoleAdmin}}
	editor := auth.User{Roles: []string{rbac.RoleEditor}}
	viewer := auth.User{Roles: []string{rbac.RoleViewer}}
	if !reg.Has(admin, rbac.Perm("items", "delete")) || !reg.Has(admin, rbac.Perm("users", "manage")) {
		t.Fatal("admin preset")
	}
	if !reg.Has(editor, rbac.Perm("items", "write")) || reg.Has(editor, rbac.Perm("items", "delete")) {
		t.Fatal("editor preset")
	}
	if !reg.Has(viewer, rbac.Perm("items", "read")) || reg.Has(viewer, rbac.Perm("items", "write")) {
		t.Fatal("viewer preset")
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

func TestFuncMapCan(t *testing.T) {
	reg := rbac.New()
	reg.Grant(rbac.RoleEditor, rbac.Perm("items", "write"))
	tmpl, err := template.New("t").Funcs(rbac.FuncMap(reg)).Parse(`{{if Can .User "items:write"}}yes{{else}}no{{end}}`)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, map[string]any{"User": auth.User{Roles: []string{rbac.RoleEditor}}}); err != nil {
		t.Fatal(err)
	}
	if b.String() != "yes" {
		t.Fatalf("got %q", b.String())
	}
}
