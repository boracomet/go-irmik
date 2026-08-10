<p align="center">
  <img src="assets/irmik.png" alt="Irmik" width="320" />
</p>

<h1 align="center">Irmik</h1>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white" alt="Go" />
  <img src="https://img.shields.io/badge/License-MIT-blue" alt="License MIT" />
  <a href="https://pkg.go.dev/github.com/boracomet/go-irmik"><img src="https://pkg.go.dev/badge/github.com/boracomet/go-irmik.svg" alt="Go Reference" /></a>
</p>

**Irmik** is a Gin-based meta-framework for Go: a **thin core** plus a **wide opt-in catalog**.  
It is built for **admin / internal systems** and **SSR sites** that want file routes, render modes, auth, SQL, and realtime — without a CMS or a megakit in every binary.

**Primary product path:** [examples/admin](examples/admin) — session + HTMX admin UI and a JWT REST `/api/v1` API (`irmik/admin`, `irmik/api`, `irmik/paginate`, `irmik/rbac`). Ready for Next.js and other SPA clients.

**Module:** [`github.com/boracomet/go-irmik`](https://github.com/boracomet/go-irmik)

> **Türkçe özet:** Irmik, Gin üzerine kurulu bir meta-framework’tür. Çekirdek ince kalır; upload, kuyruk, mail, OpenAPI gibi parçalar **isteğe bağlı** paketlerdir. Dosya tabanlı rotalar, SSR/SSG/ISR, `html/template` + React/Vite islands, Markdown içerik, auth/session, SQL/migration, SSE/WebSocket ve admin güvenlik varsayılanları sunar. Hem iç sistem / admin panelleri hem de SSR siteler için uygundur. Karşılaştırma: [docs/compare.md](docs/compare.md). Katalog: [docs/catalog.md](docs/catalog.md).

## Who is this for?

- Teams shipping **admin panels / internal tools** on Gin with CSRF, sessions, RBAC, and a JWT API for Next.js or mobile
- Sites that want **file-based routes** and per-route **SSR / SSG / ISR / Static / CSR** without leaving Go
- Projects that need **SQL + migrations**, **SSE/WebSocket**, or platform helpers (upload, queue, mail, …) **only when imported**

## Who is this NOT for?

- Pure JSON microservices that only need a router — use Gin/Echo/Fiber alone
- Anyone looking for a **CMS**, **GraphQL-in-core**, or a Beego/GoFrame-style megakit (those stay out of `irmik.New`)
- SPA-only frontends with no Go rendering story (Irmik can still serve the API; the value is SSR/admin)

## Why Irmik

- **Gin-compatible** — keep Gin handlers, middleware, and ecosystem muscle memory
- **Thin core + lean linking** — Redis, DB drivers, S3, OTel, gRPC link only when blank-imported
- **Wide opt-in catalog** — platform pieces without a batteries-forced megakit ([docs/catalog.md](docs/catalog.md))
- **SSR / SSG / ISR** — per-route modes in `_meta.yaml`, seeded by `irmik build`
- **Admin security defaults** — baseline headers; `EnableSecureDefaults()` rate limit; CSRF for cookie sessions
- **Auth + SQL + realtime** — sessions/JWT/RBAC, `database/sql` + migrate, SSE + WebSocket hubs
- **First-class CLI** — `dev`, `build`, `generate`, `start`, `migrate`, `cache clear`
- **Islands when you need interactivity** — React/Vite without turning the whole app into SPA-only

## Feature map

| Piece | Role | Docs |
|-------|------|------|
| **Routing & modes** | `app/**/page.html` + `_meta.yaml` → Gin routes; SSR/SSG/ISR/Static/CSR | below |
| **Render** | Layouts, partials, `html/template` + `tmplfunc` helpers | [architecture](docs/architecture.md) |
| **Content** | `content/<collection>/**/*.md` + frontmatter (goldmark) | [architecture](docs/architecture.md) |
| **Islands** | `{{ island "Name" … }}` via Vite + React | below |
| **Auth** | Sessions, CSRF, JWT, passwords, OAuth stubs | [auth](docs/auth.md) |
| **RBAC** | Presets, `Can`, Gin guards; optional SQL store | [rbac](docs/rbac.md) |
| **DB / migrate** | Opt-in drivers + golang-migrate–compatible files + optional GORM | [database](docs/database.md) |
| **Realtime** | SSE streams + WebSocket hub/rooms | [realtime](docs/realtime.md) |
| **Security** | Headers by default; rate limit / login limits for admin | [security](docs/security.md) |
| **Admin + API** | HTMX CRUD helpers, pagination, REST error envelope + `/api/v1` | [admin](docs/admin.md), [api](docs/api.md) |
| **Catalog** | upload, storage/S3, forms, mail, queue, scheduler, openapi, observe, compress, imagex, secrets, grpcx, proxy, testkit, audit, cors, htmx, health | [catalog](docs/catalog.md) |
| **CLI** | generate routes, dev server, build/export, migrate | below |

**Docs index:** [architecture](docs/architecture.md) · [catalog](docs/catalog.md) · [admin](docs/admin.md) · [api](docs/api.md) · [rbac](docs/rbac.md) · [auth](docs/auth.md) · [database](docs/database.md) · [realtime](docs/realtime.md) · [security](docs/security.md) · [compare](docs/compare.md) · [roadmap](docs/roadmap.md)

## Comparison

| | **Irmik** | **Gin alone** | **[StatiGo](https://github.com/Elagoht/StatiGo)** | **Echo** | **Buffalo** | **Fiber** |
|---|-----------|---------------|--------------------------------------------------|----------|-------------|-----------|
| HTTP | Gin | Gin | chi | Echo | gorilla/mux | fasthttp |
| SSR/SSG/ISR | Yes | — | Yes (static-first) | — | Templates / pipeline | — |
| Auth/RBAC | Opt-in | DIY | Site security focus | DIY | Ecosystem | DIY |
| SQL/migrate | Opt-in | DIY | — | DIY | Pop/Soda | DIY |
| Realtime | SSE + WS | DIY | — | DIY | Workers | WS DIY |
| Admin security defaults | Yes | — | Strong site defaults | DIY | Conventions | DIY |
| Opt-in catalog | Wide | — | Site kit | MW ecosystem | Batteries (heavier) | MW ecosystem |
| Lean linking | Yes | Very lean | Lean / embed | Lean | Heavier | Lean-ish |
| Best fit | Admin + SSR sites | APIs / glue | Content/SEO sites | APIs | Full-stack apps | Fast APIs |

Full notes and fair caveats: **[docs/compare.md](docs/compare.md)**. StatiGo lessons (attribution): [docs/statigo-lessons.md](docs/statigo-lessons.md).

## Quick start

```bash
go get github.com/boracomet/go-irmik@latest

# primary showcase: session admin UI + JWT REST API
cd examples/admin
go run .
# http://127.0.0.1:8080/login  →  /admin/items
# API: POST /api/v1/token  ·  CRUD /api/v1/items (Bearer)
```

Demo logins (password `password123`): `admin@example.com` (full access) · `editor@example.com` (no delete).

SSR / islands blog (optional):

```bash
cd examples/blog
npm install && npm run dev   # optional islands HMR on :5173
go run .                     # http://127.0.0.1:8080
```

Minimal app sketch:

```go
cfg, _ := config.Load("irmik.yaml")
app, _ := irmik.New(cfg)
_ = app.MountPages(irmik.MountOptions{
    Loaders: map[string]router.Loader{
        "/": irmik.AdaptLoader(func(c *irmik.Context) (any, error) {
            return map[string]any{"Title": "Home"}, nil
        }),
    },
})
_ = app.Run(ctx)
```

## Rendering modes

Set `mode` in `_meta.yaml` next to each `page.html`:

| Mode | Runtime | Build (`irmik build`) |
|------|---------|------------------------|
| `ssr` | Render every request | skipped |
| `ssg` | Serve `out/` (fallback live render) | pre-render HTML |
| `isr` | Cache HIT/STALE + background revalidate; `ETag` / `X-Cache` | seed HTML + cache |
| `static` | Prefer `out/` / `public/` | copy / pre-render |
| `csr` | Thin shell + islands | shell HTML |

```yaml
# app/blog/[slug]/_meta.yaml
mode: isr
revalidate: 60s
sitemap: true
```

## File-based routing

```text
app/
  layout.html
  page.html              → /
  _meta.yaml
  about/
    page.html            → /about
    _meta.yaml           # mode: ssg
  blog/
    page.html            → /blog
    [slug]/
      page.html          → /blog/:slug
      _meta.yaml         # mode: isr
```

Register data loaders by Gin path pattern (`/blog/:slug`) via `MountOptions.Loaders` or `Router.RegisterLoader`.

## Content collections

```go
store, _ := content.Load("content")
posts, _ := content.List[PostMeta](store, "posts")
doc, _ := content.Get[PostMeta](store, "posts", "hello-irmik")
```

`irmik build` expands dynamic routes from collections when possible (`/blog/:slug` ← `content/posts`).

## Islands

```html
<head>{{ islandRuntime }}</head>
<body>
  {{ island "Counter" (dict "initial" 0) }}
</body>
```

Dev: Vite at `islands.devServer`. Prod: `public/islands/manifest.json`. Scaffold: `templates/scaffold/`.

## CLI

```bash
go run ./cmd/irmik --help

irmik generate          # app/ → zz_routes_gen.go
irmik dev               # Gin + optional Vite + fsnotify
irmik build             # islands + SSG/ISR seed + sitemap → out/
irmik start             # production serve
irmik cache clear       # wipe configured cache store
irmik migrate up|down|status|create <name>
```

## Database

SQL via `database/sql` and golang-migrate–compatible files under `migrations/`. Drivers are **opt-in** blank-imports — see [docs/database.md](docs/database.md).

```go
import _ "github.com/boracomet/go-irmik/irmik/db/sqlite" // or postgres / mysql
```

```yaml
# irmik.yaml
database:
  driver: sqlite
  dsn: ./data/app.db
  migratePath: migrations
```

## Security

Baseline headers ship with `irmik.New`. Admin apps should also call `app.EnableSecureDefaults()` (rate limit) and use CSRF for cookie sessions.

**Before production:**

1. Set a strong **`IRMIK_JWT_SECRET`** (never ship the demo `dev-only-…` value)
2. Use **Secure session cookies** (`session.secure: true` or leave unset outside development — Secure defaults on when not `development`)
3. Terminate **HTTPS** (HSTS is emitted in production)
4. Restrict WebSocket origins and trusted proxies

Details: [docs/security.md](docs/security.md). Reporting: [SECURITY.md](SECURITY.md).

## Lean linking (optional drivers)

Heavy optional backends are **not** linked until you import them:

| Need | Import |
|------|--------|
| Cache Redis | `_ "github.com/boracomet/go-irmik/irmik/cache/redisx"` |
| Session Redis | `_ "github.com/boracomet/go-irmik/irmik/session/redisx"` |
| SQLite | `_ "github.com/boracomet/go-irmik/irmik/db/sqlite"` |
| Postgres | `_ "github.com/boracomet/go-irmik/irmik/db/postgres"` |
| MySQL | `_ "github.com/boracomet/go-irmik/irmik/db/mysql"` |

## Opt-in catalog

Platform helpers live in separate packages — **not** wired by `irmik.New`. Import only what you need:

`upload`, `storage` (+ `storage/s3x`), `forms`, `mail`, `queue`, `scheduler`, `openapi`, `observe` (+ `observe/otelx`), `compress` (+ `brotlix`), `imagex`, `secrets`, `grpcx`, `proxy`, `testkit`, `audit`, `api`, `paginate`, `admin`, `htmx`, `cors`, `health`, `rbac` (+ `rbac/store`).

Full module map: **[docs/catalog.md](docs/catalog.md)**. Admin + API guides: **[docs/admin.md](docs/admin.md)**, **[docs/api.md](docs/api.md)**.

## Realtime

SSE and WebSocket helpers for Gin. Set `server.writeTimeout: 0` for long-lived connections. See [docs/realtime.md](docs/realtime.md).

```go
app.Engine.GET("/events", sse.Handler(sse.Options{Heartbeat: 15 * time.Second}, fn))
hub := ws.NewHub(ws.Options{AllowedOrigins: cfg.Realtime.AllowedOrigins})
hub.Start()
app.Engine.GET("/ws", hub.ServeHTTP)
```

## Architecture

```mermaid
flowchart TB
  CLI["cmd/irmik CLI"] --> App["irmik.App"]
  App --> Gin["gin.Engine"]
  App --> Router["router file-based"]
  App --> Render["render html/template"]
  App --> Cache["cache memory/disk/redis"]
  App --> Content["content collections"]
  App --> SEO["seo sitemap/OG"]
  App --> Plugins["plugin hooks"]
  Router --> Modes["SSR SSG ISR Static CSR"]
  Render --> Islands["Vite React islands"]
  OptIn["opt-in: auth admin api rbac db sse ws catalog"] -.-> App
  CLI --> Build["build SSG export"]
```

See [docs/architecture.md](docs/architecture.md).

## Examples

- [`examples/admin`](examples/admin) — **start here:** HTMX Items admin + JWT `/api/v1/items` (Next.js-ready)
- [`examples/blog`](examples/blog) — SSR home, SSG about, ISR posts, Counter island
- [`examples/auth`](examples/auth) — session login, CSRF, JWT, RBAC
- [`examples/realtime`](examples/realtime) — SSE clock/stream, WebSocket echo + chat room

## Status

| Area | Status | Docs |
|------|--------|------|
| Admin UI + REST API kit | Done | [docs/admin.md](docs/admin.md), [docs/api.md](docs/api.md) |
| Auth + RBAC | Done | [docs/auth.md](docs/auth.md), [docs/rbac.md](docs/rbac.md) |
| Database / migrations | Done | [docs/database.md](docs/database.md) |
| Realtime (SSE + WebSocket) | Done | [docs/realtime.md](docs/realtime.md) |
| Lean linking + security defaults | Done | [docs/security.md](docs/security.md) |
| Opt-in feature catalog | Done | [docs/catalog.md](docs/catalog.md) |

Further ideas and the **do not add to core** list: **[docs/roadmap.md](docs/roadmap.md)**.

## Contributing

Issues and PRs welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for workflow and [SECURITY.md](SECURITY.md) for private vulnerability reports. Please do not paste secrets, tokens, or production credentials into issues.

## License

MIT. StatiGo-inspired helpers are reimplementations; see [docs/statigo-lessons.md](docs/statigo-lessons.md) for attribution.
