# Security defaults

Irmik stays lean by default, but admin-facing apps should turn on the security helpers below. Pair them with [CSRF](auth.md#csrf) for browser forms.

## Baseline (on by default)

`irmik.New` mounts recovery, request ID, and **security headers**:

| Header | Default |
|--------|---------|
| `X-Content-Type-Options` | `nosniff` |
| `X-Frame-Options` | `DENY` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=()` |
| `Strict-Transport-Security` | set in **production** (`max-age=31536000; includeSubDomains`) |
| `Content-Security-Policy` | production adds `frame-ancestors 'none'`; script policies remain opt-in so islands can use their own nonce policy |

Customize or replace with CSP framing:

```go
app.EnableSecureHeaders(middleware.SecureHeadersConfig{
    FrameAncestors: "'none'",
    FrameOptionsSkip: true,
})
```

Trusted reverse proxies (for `ClientIP` / rate limits):

```yaml
server:
  trustedProxies:
    - 10.0.0.0/8
    - 127.0.0.1
```

## Rate limiting (opt-in)

In-memory token bucket (no Redis dependency). Global + stricter login helper:

```go
app, _ := irmik.New(cfg)
app.EnableSecureDefaults() // global ~50 rps / burst 100

// Stricter on auth routes:
app.Engine.POST("/login", middleware.LoginRateLimit(), loginHandler)
```

Or configure yourself:

```go
app.EnableRateLimit(middleware.RateLimitConfig{RPS: 20, Burst: 40})
```

Keys default to `c.ClientIP()` (honors Gin trusted proxies).

## Secrets & cookies (production)

| Setting | Guidance |
|---------|----------|
| `auth.jwtSecret` / `IRMIK_JWT_SECRET` | **Required** outside development. Startup rejects empty, known demo values, and secrets shorter than 32 characters; use a long random secret. |
| `session.secure` | Cookie `Secure` flag. If omitted, Irmik sets Secure when env is **not** `development`. Force `true` behind HTTPS. |
| `session` secret / Redis URL | Prefer env (`IRMIK_SESSION_SECRET`, `REDIS_URL`); do not commit real values. |

Report vulnerabilities privately via [SECURITY.md](../SECURITY.md) — do not paste live secrets into GitHub issues.

## Admin checklist

1. `app.EnableSecureDefaults()` (or explicit `EnableRateLimit`)
2. `app.EnableSessions()` + `csrf.Middleware` for cookie/session UIs
3. Strong `IRMIK_JWT_SECRET` (never ship demo secrets)
4. HTTPS in production (headers already emit HSTS when `app.env` is not development)
5. Secure session cookies (`session.secure` / production default)
6. Blank-import only the DB/cache/session drivers you need ([database.md](database.md)). Unused imports stay out of the binary; `go get` still downloads the module graph.

## Packages

| API | Package |
|-----|---------|
| `SecureHeaders`, `RateLimit`, `LoginRateLimit` | [`irmik/middleware`](../irmik/middleware) |
| `EnableSecureDefaults`, `EnableRateLimit`, `EnableSecureHeaders` | [`irmik.App`](../irmik/app.go) |
| CSRF | [`irmik/csrf`](../irmik/csrf) |
