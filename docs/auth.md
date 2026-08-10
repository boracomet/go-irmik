# Auth stack (Phase 2.1)

Irmik’s auth layer is optional and thin: enable what you need on `irmik.App`, then mount Gin middleware.

| Package | Role |
|---------|------|
| [`irmik/validate`](../irmik/validate) | Struct/tag validation + Gin `BindJSON` / `BindForm` |
| [`irmik/session`](../irmik/session) | Cookie sessions (memory default; redis via `session/redisx`), flash, middleware |
| [`irmik/csrf`](../irmik/csrf) | CSRF token in session; validate header/form on unsafe methods |
| [`irmik/auth`](../irmik/auth) | Session login/logout, JWT HS256, password hash (argon2id/bcrypt), OAuth `Provider` |
| [`irmik/rbac`](../irmik/rbac) | Roles → permissions, presets, template `Can`, `RequireRole` / `RequirePermission` — see [rbac.md](rbac.md) |
| [`irmik/rbac/store`](../irmik/rbac/store) | Opt-in Memory/SQL persistence + `LoadRegistry` |

## Config / env

```yaml
session:
  driver: memory   # or redis (requires blank-import irmik/session/redisx)
  name: irmik_session
  maxAge: 24h
  sameSite: lax
  # secure: omitted → Secure when not development

auth:
  jwtSecret: "..."      # or IRMIK_JWT_SECRET
  jwtIssuer: irmik
  accessTTL: 15m
```

Env: `IRMIK_SESSION_DRIVER`, `IRMIK_SESSION_SECRET`, `IRMIK_SESSION_REDIS_URL`, `IRMIK_JWT_SECRET`, `IRMIK_JWT_ISSUER`, `REDIS_URL`.

### Redis sessions

Core `irmik/session` does **not** link go-redis. Enable Redis with:

```go
import _ "github.com/boracomet/go-irmik/irmik/session/redisx"
```

Then set `session.driver: redis` (or call `redisx.Register()` explicitly).

## Wiring

```go
app, _ := irmik.New(cfg)          // security headers on by default
app.EnableSecureDefaults()        // admin: global rate limit (see docs/security.md)
_ = app.EnableSessions()          // mounts session middleware
a := app.EnableAuth()             // JWT + session helpers

app.Engine.Use(a.InjectSessionUser(lookupByID))
app.Engine.Use(csrf.Middleware(csrf.Options{}))

app.Engine.POST("/login", middleware.LoginRateLimit(), func(c *gin.Context) {
    // validate.BindJSON → auth.CheckPassword → a.LoginSession(c, user)
})
app.Engine.GET("/me", a.RequireAuthSession(), handler)
app.Engine.GET("/admin", a.RequireAuthSession(), rbac.RequirePermission(reg, "users:manage"), handler)
app.Engine.GET("/api/me", a.RequireJWT(), handler)
```

`irmik.Context` exposes `Session()` and `User()` when using `irmik.Wrap` / loaders.

For admin UIs, pair CSRF with [secure defaults](security.md) (`EnableSecureDefaults`, login rate limit, HTTPS/HSTS).

## Passwords

```go
hash, _ := auth.HashPasswordDefault("secret")           // argon2id
hash, _ := auth.HashPassword(pw, auth.PasswordOptions{Algo: auth.AlgoBcrypt})
_ = auth.CheckPassword(hash, pw)
```

## JWT

```go
tok, exp, _ := a.IssueAccessToken(auth.User{ID: "1", Roles: []string{"admin"}})
claims, _ := a.ParseAccessToken(tok)
```

## OAuth

Implement `auth.Provider` (`Name`, `AuthCodeURL`, `Exchange`). Built-ins:

- `StubProvider` — local/demo exchange (`code=demo`)
- `GitHubProvider` / `GoogleProvider` — AuthCodeURL ready; `Exchange` left for your HTTP client

## CSRF

After session middleware, `csrf.Middleware` stores a token in the session. Clients send `X-CSRF-Token` or form field `_csrf`. Read with `csrf.Token(c)`.

## RBAC

Full guide: **[rbac.md](rbac.md)** (presets, SQL store, JWT/session roles, admin UI `Can`).

## Example

See [`examples/auth`](../examples/auth) and the admin showcase [`examples/admin`](../examples/admin).
