# Auth stack

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
  jwtSecret: "..."      # or IRMIK_JWT_SECRET — required for JWT; never ship demo defaults
  jwtIssuer: irmik
  accessTTL: 15m
```

Env: `IRMIK_SESSION_DRIVER`, `IRMIK_SESSION_SECRET`, `IRMIK_SESSION_REDIS_URL`, `IRMIK_JWT_SECRET`, `IRMIK_JWT_ISSUER`, `REDIS_URL`.

In production set a strong `IRMIK_JWT_SECRET` and keep session cookies Secure (see [security.md](security.md)).

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

Access tokens are HS256 JWTs. Rotating refresh tokens are optional and stored in a `RefreshStore`.

The default `MemoryRefreshStore` is **process-local**: TTL/GC in this process, gone on restart, not shared across replicas. Do not treat in-memory rotation as a production JWT story. Implement `RefreshStore` (Redis/SQL) for multi-instance deploys.

```go
tok, exp, _ := a.IssueAccessToken(auth.User{ID: "1", Roles: []string{"admin"}})
claims, _ := a.ParseAccessToken(tok)

pair, _ := a.IssueTokenPair(user)
pair, _ = a.Refresh(pair.RefreshToken, user)
a.RevokeUser(user.ID)
```

`irmik.Context.MustUser` panics if no user is in context (use `User()` when the request may be anonymous).

## OAuth

Implement `auth.Provider` (`Name`, `AuthCodeURL`, `Exchange`). Built-ins:

- `StubProvider` — local/demo exchange (`code=demo`)
- `GitHubStub` / `GoogleStub` — **not** OAuth clients. `AuthCodeURL` only builds the authorize URL; `Exchange` always returns `ErrOAuthNotImplemented`. There is no GitHub or Google flow in Irmik.

## CSRF

After session middleware, `csrf.Middleware` stores a token in the session. Clients send `X-CSRF-Token` or form field `_csrf`. Read with `csrf.Token(c)`.

## RBAC

Full guide: **[rbac.md](rbac.md)** (presets, SQL store, JWT/session roles, admin UI `Can`).

## Example

See [`examples/auth`](../examples/auth) and the admin showcase [`examples/admin`](../examples/admin).
