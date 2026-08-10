// Package rbac provides a simple role/permission registry and Gin middleware.
package rbac

import (
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/auth"
)

func sortStrings(ss []string) {
	sort.Strings(ss)
}

// Checker evaluates whether a user has a permission or role.
type Checker interface {
	HasRole(user auth.User, role string) bool
	HasPermission(user auth.User, permission string) bool
}

// Registry is an in-memory policy: roles → permissions, plus optional user grants.
type Registry struct {
	mu sync.RWMutex
	// rolePerms maps role → set of permissions
	rolePerms map[string]map[string]struct{}
	// userRoles maps user id → roles (optional overlay)
	userRoles map[string]map[string]struct{}
	// userPerms maps user id → direct permissions
	userPerms map[string]map[string]struct{}
}

// New returns an empty Registry.
func New() *Registry {
	return &Registry{
		rolePerms: make(map[string]map[string]struct{}),
		userRoles: make(map[string]map[string]struct{}),
		userPerms: make(map[string]map[string]struct{}),
	}
}

// Grant assigns permissions to a role (alias of GrantRolePermissions).
func (r *Registry) Grant(role string, perms ...string) {
	r.GrantRolePermissions(role, perms...)
}

// GrantRolePermissions assigns permissions to a role.
func (r *Registry) GrantRolePermissions(role string, perms ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	role = strings.TrimSpace(role)
	if role == "" {
		return
	}
	set, ok := r.rolePerms[role]
	if !ok {
		set = make(map[string]struct{})
		r.rolePerms[role] = set
	}
	for _, p := range perms {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		set[p] = struct{}{}
	}
}

// PermissionsFor returns a sorted copy of permissions granted to a role.
func (r *Registry) PermissionsFor(role string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	set := r.rolePerms[strings.TrimSpace(role)]
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sortStrings(out)
	return out
}

// Has is a short alias for HasPermission.
func (r *Registry) Has(user auth.User, permission string) bool {
	return r.HasPermission(user, permission)
}

// Roles returns known role names (those with at least one permission grant).
func (r *Registry) Roles() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.rolePerms))
	for role := range r.rolePerms {
		out = append(out, role)
	}
	sortStrings(out)
	return out
}

// AssignRoles gives roles to a user id.
func (r *Registry) AssignRoles(userID string, roles ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	set, ok := r.userRoles[userID]
	if !ok {
		set = make(map[string]struct{})
		r.userRoles[userID] = set
	}
	for _, role := range roles {
		set[role] = struct{}{}
	}
}

// GrantUserPermissions assigns direct permissions to a user id.
func (r *Registry) GrantUserPermissions(userID string, perms ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	set, ok := r.userPerms[userID]
	if !ok {
		set = make(map[string]struct{})
		r.userPerms[userID] = set
	}
	for _, p := range perms {
		set[p] = struct{}{}
	}
}

// RolesFor returns effective roles for a user (context roles ∪ registry).
func (r *Registry) RolesFor(user auth.User) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[string]struct{})
	var out []string
	add := func(role string) {
		role = strings.TrimSpace(role)
		if role == "" {
			return
		}
		if _, ok := seen[role]; ok {
			return
		}
		seen[role] = struct{}{}
		out = append(out, role)
	}
	for _, role := range user.Roles {
		add(role)
	}
	for role := range r.userRoles[user.ID] {
		add(role)
	}
	return out
}

// HasRole implements Checker.
func (r *Registry) HasRole(user auth.User, role string) bool {
	for _, got := range r.RolesFor(user) {
		if got == role {
			return true
		}
	}
	return false
}

// HasPermission implements Checker.
func (r *Registry) HasPermission(user auth.User, permission string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if perms, ok := r.userPerms[user.ID]; ok {
		if _, ok := perms[permission]; ok {
			return true
		}
	}
	roles := make([]string, 0, len(user.Roles)+len(r.userRoles[user.ID]))
	seen := map[string]struct{}{}
	for _, role := range user.Roles {
		if _, ok := seen[role]; !ok {
			seen[role] = struct{}{}
			roles = append(roles, role)
		}
	}
	for role := range r.userRoles[user.ID] {
		if _, ok := seen[role]; !ok {
			seen[role] = struct{}{}
			roles = append(roles, role)
		}
	}
	for _, role := range roles {
		if perms, ok := r.rolePerms[role]; ok {
			if _, ok := perms[permission]; ok {
				return true
			}
		}
	}
	return false
}

// RequireRole aborts with 403 unless the authenticated user has one of the roles.
func RequireRole(checker Checker, roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := auth.UserFrom(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		for _, role := range roles {
			if checker.HasRole(u, role) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	}
}

// RequirePermission aborts with 403 unless the user has the permission.
func RequirePermission(checker Checker, permission string) gin.HandlerFunc {
	return RequireAnyPermission(checker, permission)
}

// RequireAnyPermission aborts with 403 unless the user has at least one permission.
func RequireAnyPermission(checker Checker, permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := auth.UserFrom(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		for _, permission := range permissions {
			if checker.HasPermission(u, permission) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	}
}

// FuncChecker adapts functions into a Checker.
type FuncChecker struct {
	RoleFn func(user auth.User, role string) bool
	PermFn func(user auth.User, permission string) bool
}

// HasRole implements Checker.
func (f FuncChecker) HasRole(user auth.User, role string) bool {
	if f.RoleFn == nil {
		return false
	}
	return f.RoleFn(user, role)
}

// HasPermission implements Checker.
func (f FuncChecker) HasPermission(user auth.User, permission string) bool {
	if f.PermFn == nil {
		return false
	}
	return f.PermFn(user, permission)
}
