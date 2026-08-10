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

## Admin checklist

1. `app.EnableSecureDefaults()` (or explicit `EnableRateLimit`)
2. `app.EnableSessions()` + `csrf.Middleware` for cookie/session UIs
3. HTTPS in production (headers already emit HSTS when `app.env` is not development)
4. Secure session cookies (`session.secure` / production default)
5. Blank-import only the DB/cache/session drivers you need ([database.md](database.md), README lean linking)

## Packages

| API | Package |
|-----|---------|
| `SecureHeaders`, `RateLimit`, `LoginRateLimit` | [`irmik/middleware`](../irmik/middleware) |
| `EnableSecureDefaults`, `EnableRateLimit`, `EnableSecureHeaders` | [`irmik.App`](../irmik/app.go) |
| CSRF | [`irmik/csrf`](../irmik/csrf) |
