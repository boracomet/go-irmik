package rbac

import "strings"

// Role is a named role identifier (e.g. "admin").
type Role = string

// Permission is a capability string, conventionally "resource:action".
type Permission = string

// Built-in role name presets. Apps may use these or define their own.
const (
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

// Perm builds a permission name: Perm("posts", "read") → "posts:read".
// Convention: lowercase resource and action separated by a single colon.
func Perm(resource, action string) Permission {
	return strings.TrimSpace(resource) + ":" + strings.TrimSpace(action)
}

// ResourcePerms returns read/write/delete permissions for a resource.
func ResourcePerms(resource string) (read, write, delete Permission) {
	return Perm(resource, "read"), Perm(resource, "write"), Perm(resource, "delete")
}

// PresetPermissions returns cloneable default permission sets for Admin / Editor / Viewer.
//
// For each resource, grants:
//   - Viewer:  resource:read
//   - Editor:  resource:read, resource:write
//   - Admin:   resource:read, resource:write, resource:delete
//
// Admin also receives "users:manage" and "dashboard:read".
// Editor and Viewer receive "dashboard:read".
//
// Callers typically copy and extend:
//
//	presets := rbac.PresetPermissions("items", "posts")
//	presets[rbac.RoleEditor] = append(presets[rbac.RoleEditor], rbac.Perm("media", "upload"))
func PresetPermissions(resources ...string) map[Role][]Permission {
	admin := []Permission{Perm("dashboard", "read"), Perm("users", "manage")}
	editor := []Permission{Perm("dashboard", "read")}
	viewer := []Permission{Perm("dashboard", "read")}
	for _, res := range resources {
		res = strings.TrimSpace(res)
		if res == "" {
			continue
		}
		read, write, del := ResourcePerms(res)
		viewer = append(viewer, read)
		editor = append(editor, read, write)
		admin = append(admin, read, write, del)
	}
	return map[Role][]Permission{
		RoleAdmin:  admin,
		RoleEditor: editor,
		RoleViewer: viewer,
	}
}

// ApplyPresets grants the built-in Admin / Editor / Viewer permission sets for resources.
func (r *Registry) ApplyPresets(resources ...string) {
	for role, perms := range PresetPermissions(resources...) {
		r.Grant(role, perms...)
	}
}
