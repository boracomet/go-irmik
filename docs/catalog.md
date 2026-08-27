# Feature catalog

Irmik’s **core promise** is file-based `app/` routes, SSR/SSG/ISR render modes, and security-minded admin helpers (session, CSRF, RBAC, HTMX) that you opt into.

This catalog is **frozen** (no new packages). Heavy backends that used to live in the root `go.mod` are nested modules so a core `go get` does not download them.

## Modules

| `go get` | What you download |
|----------|-------------------|
| `github.com/boracomet/go-irmik` | Root module: core + opt-in packages that stay in-tree (sessions, db drivers, migrate, Redis, brotli, imagex, …). **Not** AWS, GORM, OTel SDK, gRPC, or asynq. |
| `github.com/boracomet/go-irmik/irmik/db/gormx` | GORM helper |
| `github.com/boracomet/go-irmik/irmik/storage/s3x` | AWS SDK v2 S3 store |
| `github.com/boracomet/go-irmik/irmik/observe/otelx` | OpenTelemetry SDK bootstrap (experimental) |
| `github.com/boracomet/go-irmik/irmik/grpcx` | gRPC server/client helpers |
| `github.com/boracomet/go-irmik/irmik/queue/asynqx` | hibiken/asynq queue driver |

Import paths are the same as before (`irmik/storage/s3x`, …). After v0.2.0, nested modules are tagged as `irmik/<path>/v0.2.0` (for example `irmik/db/gormx/v0.2.0`). Until those tags exist, pin `@main` or a commit.

A couple of tiny OpenTelemetry API modules (`otel/trace`, `otelhttp`) may still appear as **indirect** root deps via golang-migrate’s test graph (Docker). That is not the OTel SDK and is not `otelx`.

## How linking works

| Import style | When to use | Downloaded with root `go get`? | Linked into binary? |
|--------------|-------------|--------------------------------|---------------------|
| Nothing extra | Core SSR/SSG/ISR app | Root module only | Only packages you already use |
| Explicit `Open` / `New` | `storage.OpenLocal`, `mail.NewSMTP`, `queue.NewMemory` | Yes (root) | Yes, that package + its deps |
| Nested module | `s3x.Open`, `gormx.Open`, `asynqx.Open`, `otelx.Setup`, `grpcx.NewServer` | **No** — `go get` the nested path | Yes, once imported |
| Blank-import register | `import _ "…/cache/redisx"` | Yes (root) | Yes (registers driver) |
| Blank-import asynq | `import _ "…/queue/asynqx"` | **No** — nested module | Yes (registers driver) |
| In-root heavy-ish | `compress/brotlix`, `imagex`, db drivers | Yes (root) | Only if you import them |

**Rule:** never hard-import nested catalog modules into root `irmik` or `irmik.New`. Prefer small APIs — no DI container.

## Core (always available if you use `irmik.New`)

| Package | Role |
|---------|------|
| `irmik` | `App`, lifecycle, loaders |
| `irmik/config` | `irmik.yaml` + env |
| `irmik/router` | File-based routes + modes |
| `irmik/render` | `html/template` |
| `irmik/island` | Vite/React islands |
| `irmik/content` | Markdown collections |
| `irmik/seo` | OG/JSON-LD/sitemap |
| `irmik/cache` | Memory/disk interfaces (+ optional Redis via `cache/redisx`) |
| `irmik/middleware` | Recovery, request ID, health/ready, opt-in `RequestLog` |
| `irmik/plugin` | Lifecycle hooks |
| `irmik/tmplfunc`, `slug`, `fsutil`, `meta`, `lifecycle` | Shared helpers |
| `irmik/devtools` | Dev only: overlay badge, errors, live reload (`irmik.New` when `IsDev`) |

## Opt-in: auth & data

| Package | Import | What gets linked |
|---------|--------|------------------|
| `irmik/session` | explicit | Cookie sessions (memory) |
| `irmik/session/redisx` | blank or `New` | go-redis session store |
| `irmik/csrf` | explicit | CSRF (uses session) |
| `irmik/auth`, `irmik/rbac` | explicit | JWT/passwords/OAuth stubs/RBAC (presets, `Can`, Gin guards) |
| `irmik/rbac/store` | explicit | Memory/SQL role persistence + `LoadRegistry` (not in core) |
| `irmik/validate` | explicit | go-playground/validator |
| `irmik/db` | explicit | `database/sql` open |
| `irmik/db/postgres`, `sqlite`, `mysql` | blank/register | Driver-specific deps |
| `irmik/db/gormx` | explicit **nested module** | GORM |
| `irmik/migrate` | explicit / CLI | golang-migrate |
| `irmik/sse`, `irmik/ws` | explicit | Realtime helpers |

## Opt-in: admin / platform catalog

| Package | Status | Import example | Heavy deps |
|---------|--------|----------------|------------|
| `irmik/upload` | Stable | `upload.Save(c, upload.Options{…})` | none |
| `irmik/storage` | Stable | `storage.OpenLocal("./data")` | none |
| `irmik/storage/s3x` | Stable | nested module: `go get …/irmik/storage/s3x` then `s3x.Open` | AWS SDK v2 |
| `irmik/forms` | Stable | `forms.BindForm` + `forms.CSRFInput(token)` | validate only (no session) |
| `irmik/mail` | Stable | `mail.NewSMTP(cfg)` / `mail.Memory` | `net/smtp` |
| `irmik/queue` | Stable | `queue.NewMemory(64)` + `Run` | none |
| `irmik/queue/asynqx` | Stable | nested module: blank-import or `asynqx.Open(opts)` | hibiken/asynq + Redis |
| `irmik/scheduler` | Stable | `scheduler.New()` + `Every` / `AddCron` / `AddCronTZ` | robfig/cron/v3 (scheduler only) |
| `irmik/openapi` | Experimental | `openapi.New(…).Mount` + `MountSwagger` | none (Swagger UI via CDN) |
| `irmik/observe` | Stable | `observe.NewLogger(opts)` | `log/slog` |
| `irmik/observe/otelx` | Experimental | nested module: `otelx.Setup(ctx, opts)` | OpenTelemetry SDK |
| `irmik/compress` | Stable | `r.Use(compress.Gzip())` | stdlib gzip |
| `irmik/compress/brotlix` | Stable | `r.Use(brotlix.Brotli())` | andybalholm/brotli |
| `irmik/imagex` | Stable | `Pipeline` (`{{ img }}` + `/_irmik/img`) · `Variants` / `WriteVariants` · `Transform` | x/image + deepteams/webp (pure Go) |
| `irmik/secrets` | Stable | `secrets.Env{Prefix:"IRMIK_"}` | none |
| `irmik/grpcx` | Stable | nested module: `grpcx.NewServer(opts).ListenAndServe(ctx)` | google.golang.org/grpc |
| `irmik/proxy` | Stable | `proxy.Handler(proxy.Options{Target: "…"})` | `httputil` |
| `irmik/testkit` | Stable | `testkit.New(t).GET("/").Do()` | gin test |
| `irmik/audit` | Stable | `audit.Record` + `audit.Middleware(sink)` | slog / memory |
| `irmik/cors` | Stable | `r.Use(cors.Middleware(opts))` | none |
| `irmik/htmx` | Stable | `htmx.IsRequest` / `Redirect` / `Trigger` / `RenderPartial` | none |
| `irmik/health` | Stable | `health.New()` + `app.RegisterReadyCheck` | none |
| `irmik/api` | Stable | `api.JSON` / `Error` / `Abort` / `V1` / `MountV1` | none |
| `irmik/paginate` | Stable | `paginate.Parse` + `OrderBy` whitelist | none |
| `irmik/admin` | Stable | flash↔HX, CRUD snippets via `ParseTemplates` / `TemplatesFS` | htmx, session, paginate |

## Wiring snippets

### Upload + local storage

```go
import (
    "github.com/boracomet/go-irmik/irmik/storage"
    "github.com/boracomet/go-irmik/irmik/upload"
)

store, _ := storage.OpenLocal("./data/blobs")
app.Engine.POST("/upload", upload.Handler(upload.Options{
    DestDir:     "./data/uploads",
    MaxBytes:    8 << 20,
    AllowedMIME: []string{"image/*", "application/pdf"},
}))
_ = store
```

### S3 (opt-in SDK)

```go
import "github.com/boracomet/go-irmik/irmik/storage/s3x" // go get this nested module

store, err := s3x.Open(ctx, s3x.Options{
    Bucket: "my-bucket", Region: "us-east-1",
    Endpoint: "http://127.0.0.1:9000", PathStyle: true, // MinIO
})
```

### Forms + CSRF field (session stays in handler)

```go
import (
    "github.com/boracomet/go-irmik/irmik/csrf"
    "github.com/boracomet/go-irmik/irmik/forms"
)

// template data:
//   "CSRF": forms.CSRFInput(csrf.Token(c))
type Input struct {
    Email string `form:"email" validate:"required,email"`
}
var in Input
if !forms.MustBindForm(c, &in) { return }
```

### Queue + scheduler

```go
q := queue.NewMemory(128)
go q.Run(ctx, func(ctx context.Context, job queue.Job) error { /* … */ return nil })
_ = q.Enqueue(ctx, queue.Job{Name: "email.welcome", Payload: payload})

sched := scheduler.New()
_ = sched.Add(scheduler.Job{Name: "cleanup", Every: time.Hour, Fn: cleanup})
_ = sched.AddCronTZ("daily-report", "0 9 * * *", "Europe/Istanbul", report)
go sched.Run(ctx)
```

### CORS / request log / audit / HTMX / ready checks

```go
import (
    "github.com/boracomet/go-irmik/irmik/audit"
    "github.com/boracomet/go-irmik/irmik/cors"
    "github.com/boracomet/go-irmik/irmik/health"
    "github.com/boracomet/go-irmik/irmik/htmx"
)

app.Engine.Use(cors.Middleware(cors.Options{AllowOrigins: []string{"https://admin.example.com"}}))
app.UseRequestLog()
app.Engine.Use(audit.Middleware(audit.Logger{})) // or &audit.Memory{}
app.RegisterReadyCheck("db", health.PingDB(db))

app.Engine.POST("/users", func(c *gin.Context) {
    if htmx.IsRequest(c) {
        htmx.Trigger(c, "userSaved")
        _ = htmx.RenderPartial(c, rowTmpl, "row", data)
        return
    }
    htmx.Redirect(c, "/users")
})
```

### REST API + admin list helpers

```go
import (
    "github.com/boracomet/go-irmik/irmik/api"
    "github.com/boracomet/go-irmik/irmik/admin"
    "github.com/boracomet/go-irmik/irmik/paginate"
)

api.MountV1(app.Engine, func(v1 *gin.RouterGroup) {
    v1.Use(authenticator.RequireJWT())
    v1.GET("/items", func(c *gin.Context) {
        p := paginate.Parse(c, paginate.Options{SortWhitelist: []string{"id", "title"}})
        // use p.Offset(), p.Limit(), p.OrderBy(columns)
        api.JSON(c, 200, gin.H{"data": items, "meta": gin.H{"page": p.Page}})
    })
})

admin.FlashHX(c, admin.FlashSuccess, "Saved")
tmpl, _ := admin.ParseTemplates(nil) // list.html, form.html, confirm_delete.html, flash.html
```

Docs: [api.md](api.md), [admin.md](admin.md), [rbac.md](rbac.md). Showcase: [examples/admin](../examples/admin).

### Asynq / Redis queue (opt-in)

```go
import (
    "github.com/boracomet/go-irmik/irmik/queue"
    _ "github.com/boracomet/go-irmik/irmik/queue/asynqx" // nested module; registers "asynq"
)

q, err := queue.New(queue.Options{
    Driver: "asynq", RedisURL: "redis://localhost:6379/0", Concurrency: 10,
})
// or: q, err := asynqx.Open(asynqx.Options{RedisURL: "redis://localhost:6379/0"})
go q.Run(ctx, handler)
_ = q.Enqueue(ctx, queue.Job{Name: "email.welcome", Payload: payload})
```

### OpenAPI + Swagger UI (CDN)

```go
doc := openapi.New("API", "1.0.0")
doc.Add("/users", "GET", openapi.Operation{Summary: "List users"})
doc.Mount(r, "/openapi.json")
openapi.MountSwagger(r, "/docs", "/openapi.json") // loads swagger-ui from unpkg CDN
```

Offline alternative: vendor [swagger-ui-dist](https://www.npmjs.com/package/swagger-ui-dist) under `public/` and point it at the same `/openapi.json`.

### Observe + optional OTel

```go
log := observe.NewLogger(observe.Options{Service: "api", JSON: true})
observe.SetDefault(log)

// only when you want OTel:
// shutdown, _ := otelx.Setup(ctx, otelx.Options{Service: "api"})
// defer shutdown(context.Background())
```

### gRPC

```go
srv := grpcx.NewServer(grpcx.ServerOptions{
    Addr: ":50051", EnableHealth: true, EnableReflection: true,
    Register: func(s *grpc.Server) { /* pb.Register… */ },
})
go srv.ListenAndServe(ctx)
```

### Dev overlay

When `cfg.IsDev()`, `irmik.New` injects a bottom-left badge into HTML. Click it for template/JS errors, file routes, and server info. `irmik dev` live-reloads the tab after `app/` or `templates/` saves. Island TSX errors stay in the Vite overlay. Production (`irmik start` / `env: production`) does not mount this.

### Image transform (incl. WebP encode)

```go
out, ct, err := imagex.Transform(r, imagex.Options{
    MaxWidth: 800, Format: imagex.WEBP, Quality: 80,
})
```

### Responsive images (SSR frontend)

Opt-in. Not wired by `irmik.New`. Local files under `Root` only — no remote URL proxy.

```go
img := &imagex.Pipeline{Root: "./public"} // allowlist 375 / 768 / 1440
img.Mount(app.Engine)
app.MountPages(irmik.MountOptions{
    Funcs: img.FuncMap(),
})
```

```html
{{ img "/hero.jpg" (dict "alt" "Hero" "width" 1440 "height" 810 "priority" true) }}
```

`priority` is for the LCP/hero image (`fetchpriority=high`, no `loading=lazy`). Other images lazy-load.

### Upload variants (admin)

Same encoder, at `Save()` time:

```go
f, err := upload.Save(c, upload.Options{DestDir: "./data/uploads"})
raw, err := os.ReadFile(f.Path)
paths, err := imagex.WriteVariants("./data/uploads", f.Filename, bytes.NewReader(raw),
    []int{375, 1440}, imagex.Options{Format: imagex.WEBP, Quality: 80})
```

Writes `name-375.webp` and `name-1440.webp` next to the original.

Prioritized “what else / what not to add to core”: **[roadmap.md](roadmap.md)**. Positioning vs Gin / StatiGo / Echo / Buffalo / Fiber: **[compare.md](compare.md)**.

See also [architecture.md](architecture.md), [auth.md](auth.md), [rbac.md](rbac.md), [database.md](database.md), [realtime.md](realtime.md), [api.md](api.md), [admin.md](admin.md), [devtools.md](devtools.md).
