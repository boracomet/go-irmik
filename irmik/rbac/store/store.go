// Package store provides opt-in persistence for irmik/rbac.
//
// Import explicitly — this package is never pulled in by irmik.New.
// Use Memory for tests/demos, or SQL over database/sql (blank-import your driver).
package store

import (
	"context"

	"github.com/boracomet/go-irmik/irmik/rbac"
)

// Store loads and optionally mutates roles, permissions, and user-role assignments.
type Store interface {
	// EnsureSchema creates RBAC tables when missing (SQL stores).
	EnsureSchema(ctx context.Context) error
	// RolePermissions returns role → permissions.
	RolePermissions(ctx context.Context) (map[string][]string, error)
	// UserRoles returns user_id → roles.
	UserRoles(ctx context.Context) (map[string][]string, error)
	// UpsertRole inserts or updates a role row.
	UpsertRole(ctx context.Context, name, description string) error
	// UpsertPermission inserts or updates a permission row.
	UpsertPermission(ctx context.Context, name, description string) error
	// Grant attaches a permission to a role (idempotent).
	Grant(ctx context.Context, role, permission string) error
	// AssignRole attaches a role to a user id (idempotent).
	AssignRole(ctx context.Context, userID, role string) error
}

// LoadRegistry builds an in-memory rbac.Registry from a Store.
func LoadRegistry(ctx context.Context, s Store) (*rbac.Registry, error) {
	if s == nil {
		return rbac.New(), nil
	}
	rolePerms, err := s.RolePermissions(ctx)
	if err != nil {
		return nil, err
	}
	userRoles, err := s.UserRoles(ctx)
	if err != nil {
		return nil, err
	}
	reg := rbac.New()
	for role, perms := range rolePerms {
		reg.Grant(role, perms...)
	}
	for userID, roles := range userRoles {
		reg.AssignRoles(userID, roles...)
	}
	return reg, nil
}

// SeedPresets writes Admin / Editor / Viewer presets for the given resources.
// Existing grants are left in place (upsert + grant are idempotent).
func SeedPresets(ctx context.Context, s Store, resources ...string) error {
	if err := s.EnsureSchema(ctx); err != nil {
		return err
	}
	presets := rbac.PresetPermissions(resources...)
	descs := map[string]string{
		rbac.RoleAdmin:  "Full access",
		rbac.RoleEditor: "Create and edit content",
		rbac.RoleViewer: "Read-only access",
	}
	for role, perms := range presets {
		if err := s.UpsertRole(ctx, role, descs[role]); err != nil {
			return err
		}
		for _, p := range perms {
			if err := s.UpsertPermission(ctx, p, ""); err != nil {
				return err
			}
			if err := s.Grant(ctx, role, p); err != nil {
				return err
			}
		}
	}
	return nil
}
