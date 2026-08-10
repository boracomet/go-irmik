# Roles & permissions (RBAC)

Opt-in primitives for admin panels and APIs. Keep policies in `irmik/rbac`; persist with `irmik/rbac/store` only when you need SQL.

| Package | Role |
|---------|------|
| [`irmik/rbac`](../irmik/rbac) | In-memory `Registry`, Gin guards, template `Can`, role presets |
| [`irmik/rbac/store`](../irmik/rbac/store) | Opt-in Memory / SQL store + `LoadRegistry` (not linked by `irmik.New`) |

Pair with [`irmik/auth`](auth.md): session users and JWT claims both carry `Roles []string`.

## Permission naming

Convention: `resource:action` (lowercase).

```go
rbac.Perm("posts", "read")  // → "posts:read"
read, write, del := rbac.ResourcePerms("items")
```

## Registry API

```go
reg := rbac.New()
reg.Grant(rbac.RoleEditor, rbac.Perm("items", "read"), rbac.Perm("items", "write"))
reg.Grant(rbac.RoleAdmin, rbac.Perm("items", "delete"), rbac.Perm("users", "manage"))
// or cloneable presets:
reg.ApplyPresets("items", "posts") // admin / editor / viewer

reg.Has(user, "items:write")
reg.HasRole(user, rbac.RoleEditor)
reg.PermissionsFor(rbac.RoleEditor)
```

Built-in role name constants: `RoleAdmin`, `RoleEditor`, `RoleViewer`.  
`PresetPermissions(resources...)` returns a `map[role][]perm` you can copy and extend before granting.

Effective roles for a user = `user.Roles` ∪ registry `AssignRoles(userID, …)` overlay.  
Direct user permissions: `GrantUserPermissions(userID, …)`.

## Gin middleware

```go
admin.Use(a.RequireAuthSession(), rbac.RequirePermission(reg, rbac.Perm("items", "read")))
admin.DELETE("/items/:id", rbac.RequirePermission(reg, rbac.Perm("items", "delete")), deleteHandler)
// any of:
rbac.RequireAnyPermission(reg, "items:write", "items:delete")
rbac.RequireRole(reg, rbac.RoleAdmin, rbac.RoleEditor)
```

## Templates / HTMX

```go
tmpl := template.New("app").Funcs(rbac.FuncMap(reg)).ParseFS(fs, "templates/*.html")
```

```html
{{if Can .User "items:delete"}}
  <a href="/admin/items/{{.ID}}/delete">Delete</a>
{{end}}
{{if HasRole .User "admin"}}…{{end}}
```

Always enforce the same permission on the route — UI hiding is not authorization.

## Session roles

```go
a.LoginSession(c, auth.User{ID: u.ID, Email: u.Email, Roles: u.Roles})
app.Engine.Use(a.InjectSessionUser(func(id string) (*auth.User, error) {
    // load Roles from your user table (or rely on store user_roles + AssignRoles)
    return &auth.User{ID: id, Roles: roles}, nil
}))
```

## JWT claim roles

`IssueAccessToken` embeds `roles` in claims; `RequireJWT` / `MiddlewareJWT` restore them on `auth.User`.

```go
tok, _, _ := a.IssueAccessToken(auth.User{ID: "1", Roles: []string{rbac.RoleEditor}})
// Authorization: Bearer <tok>
```

Permission checks use those claim roles against the Registry (no DB hit per request once loaded).

## SQL store (opt-in)

```go
import (
    rbacstore "github.com/boracomet/go-irmik/irmik/rbac/store"
    _ "github.com/boracomet/go-irmik/irmik/db/sqlite" // or postgres
)

s := rbacstore.NewSQL(db, "sqlite") // or "postgres"
_ = rbacstore.SeedPresets(ctx, s, "items")
_ = s.AssignRole(ctx, userID, rbac.RoleEditor)
reg, err := s.LoadRegistry(ctx) // → *rbac.Registry
```

### Schema

Embedded under `irmik/rbac/store/migrations/` (also via `store.MigrationsFS()` for `irmik/migrate`):

| Table | Purpose |
|-------|---------|
| `roles` | `name` PK, `description` |
| `permissions` | `name` PK, `description` |
| `role_permissions` | `(role_name, permission_name)` |
| `user_roles` | `(user_id, role_name)` — `user_id` is TEXT (auth-agnostic) |

Memory store: `rbacstore.NewMemory()` for tests/demos without SQL.

Reload the registry after policy changes (or cache + invalidate). Typical boot path: migrate → seed → `LoadRegistry` → wire Gin + `FuncMap`.

## Modeling an admin panel

1. Name resources (`items`, `users`, …) and actions (`read` / `write` / `delete`).
2. Start from `ApplyPresets` or `PresetPermissions`, then extend.
3. Put `RequirePermission` on mutating routes; use `Can` to hide buttons.
4. Put role names on the session/JWT user; keep the permission matrix in Registry (memory or SQL-loaded).
5. Keep `irmik/rbac/store` behind an explicit import so API-only binaries stay lean.

## Example

See [`examples/admin`](../examples/admin): SQLite-seeded admin vs editor, delete guarded by `items:delete`, Delete button gated with `Can`.
