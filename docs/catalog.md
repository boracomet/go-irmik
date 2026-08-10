# Feature catalog — wide catalog, thin core

Irmik keeps **`irmik.New` and the root `irmik` package thin**. Extra capabilities live in **opt-in packages** under `irmik/<name>/`. Import only what you need so Redis, S3, OpenTelemetry, gRPC, image codecs, etc. stay out of default binaries.

## How linking works

| Import style | When to use | Linked into binary? |
|--------------|-------------|---------------------|
| Nothing | Core SSR/SSG/ISR app | Only packages you already use |
| Explicit `Open` / `New` | `storage.OpenLocal`, `mail.NewSMTP`, `queue.NewMemory`, `asynqx.Open` | Yes, that package + its deps |
| Blank-import register | `import _ "…/cache/redisx"`, `import _ "…/queue/asynqx"` | Yes (registers driver) |
| Heavy subpackage | `storage/s3x`, `observe/otelx`, `compress/brotlix`, `grpcx` | Only if you import them |

**Rule:** never hard-import heavy deps into root `irmik` or into always-on `irmik.New`. Prefer small APIs — no DI container.

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

## Opt-in: auth & data (Phase 2)

| Package | Import | What gets linked |
|---------|--------|------------------|
| `irmik/session` | explicit | Cookie sessions (memory) |
| `irmik/session/redisx` | blank or `New` | go-redis session store |
| `irmik/csrf` | explicit | CSRF (uses session) |
| `irmik/auth`, `irmik/rbac` | explicit | JWT/passwords/OAuth stubs/RBAC |
| `irmik/validate` | explicit | go-playground/validator |
| `irmik/db` | explicit | `database/sql` open |
| `irmik/db/postgres`, `sqlite`, `mysql` | blank/register | Driver-specific deps |
| `irmik/db/gormx` | explicit | GORM |
| `irmik/migrate` | explicit / CLI | golang-migrate |
| `irmik/sse`, `irmik/ws` | explicit | Realtime helpers |

## Opt-in: admin / platform catalog

| Package | Status | Import example | Heavy deps |
|---------|--------|----------------|------------|
| `irmik/upload` | Stable | `upload.Save(c, upload.Options{…})` | none |
| `irmik/storage` | Stable | `storage.OpenLocal("./data")` | none |
| `irmik/storage/s3x` | Stable | `s3x.Open(ctx, s3x.Options{Bucket: "…"})` | AWS SDK v2 |
| `irmik/forms` | Stable | `forms.BindForm` + `forms.CSRFInput(token)` | validate only (no session) |
| `irmik/mail` | Stable | `mail.NewSMTP(cfg)` / `mail.Memory` | `net/smtp` |
| `irmik/queue` | Stable | `queue.NewMemory(64)` + `Run` | none |
| `irmik/queue/asynqx` | Stable | blank-import or `asynqx.Open(opts)` | hibiken/asynq + Redis |
| `irmik/scheduler` | Stable | `scheduler.New()` + `Every` / `AddCron` / `AddCronTZ` | robfig/cron/v3 (scheduler only) |
| `irmik/openapi` | Experimental | `openapi.New(…).Mount` + `MountSwagger` | none (Swagger UI via CDN) |
| `irmik/observe` | Stable | `observe.NewLogger(opts)` | `log/slog` |
| `irmik/observe/otelx` | Experimental | `otelx.Setup(ctx, opts)` | OpenTelemetry SDK |
| `irmik/compress` | Stable | `r.Use(compress.Gzip())` | stdlib gzip |
| `irmik/compress/brotlix` | Stable | `r.Use(brotlix.Brotli())` | andybalholm/brotli |
| `irmik/imagex` | Stable | `imagex.Transform(r, opts)` incl. WebP encode | x/image + deepteams/webp (pure Go) |
| `irmik/secrets` | Stable | `secrets.Env{Prefix:"IRMIK_"}` | none |
| `irmik/grpcx` | Stable | `grpcx.NewServer(opts).ListenAndServe(ctx)` | google.golang.org/grpc |
| `irmik/proxy` | Stable | `proxy.Handler(proxy.Options{Target: "…"})` | `httputil` |
| `irmik/testkit` | Stable | `testkit.New(t).GET("/").Do()` | gin test |
| `irmik/audit` | Stable | `audit.Record` + `audit.Middleware(sink)` | slog / memory |
| `irmik/cors` | Stable | `r.Use(cors.Middleware(opts))` | none |
| `irmik/htmx` | Stable | `htmx.IsRequest` / `Redirect` / `Trigger` / `RenderPartial` | none |
| `irmik/health` | Stable | `health.New()` + `app.RegisterReadyCheck` | none |

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
import "github.com/boracomet/go-irmik/irmik/storage/s3x"

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

### Asynq / Redis queue (opt-in)

```go
import (
    "github.com/boracomet/go-irmik/irmik/queue"
    _ "github.com/boracomet/go-irmik/irmik/queue/asynqx" // registers "asynq"
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

### Image transform (incl. WebP encode)

```go
out, ct, err := imagex.Transform(r, imagex.Options{
    MaxWidth: 800, Format: imagex.WEBP, Quality: 80,
})
```

Prioritized “what else / what not to add to core”: **[roadmap.md](roadmap.md)**. Positioning vs Gin / StatiGo / Echo / Buffalo / Fiber: **[compare.md](compare.md)**.

See also [architecture.md](architecture.md), [auth.md](auth.md), [database.md](database.md), [realtime.md](realtime.md).
