package store_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/boracomet/go-irmik/irmik/auth"
	"github.com/boracomet/go-irmik/irmik/rbac"
	"github.com/boracomet/go-irmik/irmik/rbac/store"

	_ "github.com/boracomet/go-irmik/irmik/db/sqlite"
)

func TestMemorySeedAndLoad(t *testing.T) {
	ctx := context.Background()
	mem := store.NewMemory()
	if err := store.SeedPresets(ctx, mem, "items"); err != nil {
		t.Fatal(err)
	}
	if err := mem.AssignRole(ctx, "1", rbac.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if err := mem.AssignRole(ctx, "2", rbac.RoleEditor); err != nil {
		t.Fatal(err)
	}
	reg, err := store.LoadRegistry(ctx, mem)
	if err != nil {
		t.Fatal(err)
	}
	admin := auth.User{ID: "1"}
	editor := auth.User{ID: "2"}
	if !reg.Has(admin, rbac.Perm("items", "delete")) {
		t.Fatal("admin should delete")
	}
	if reg.Has(editor, rbac.Perm("items", "delete")) {
		t.Fatal("editor should not delete")
	}
	if !reg.Has(editor, rbac.Perm("items", "write")) {
		t.Fatal("editor should write")
	}
}

func TestSQLStoreSQLite(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", "file:rbac-store-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	s := store.NewSQL(db, "sqlite")
	if err := store.SeedPresets(ctx, s, "items"); err != nil {
		t.Fatal(err)
	}
	if err := s.AssignRole(ctx, "1", rbac.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if err := s.AssignRole(ctx, "2", rbac.RoleEditor); err != nil {
		t.Fatal(err)
	}

	reg, err := s.LoadRegistry(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reg.HasRole(auth.User{ID: "1"}, rbac.RoleAdmin) {
		t.Fatal("missing admin role assignment")
	}
	perms := reg.PermissionsFor(rbac.RoleEditor)
	if len(perms) == 0 {
		t.Fatal("expected editor permissions")
	}
	foundWrite := false
	for _, p := range perms {
		if p == rbac.Perm("items", "write") {
			foundWrite = true
		}
		if p == rbac.Perm("items", "delete") {
			t.Fatal("editor preset must not include delete")
		}
	}
	if !foundWrite {
		t.Fatal("editor missing write")
	}
}
