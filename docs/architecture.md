# Architecture (Phase 1)

Irmik is a thin meta-framework around Gin. The public SDK lives under `irmik/`; CLI and build tooling under `cmd/` and `internal/`.

## Request path (SSR / ISR)

1. Gin matches a file-based route from `app/` discovery.
2. Optional **loader** returns view data (`MountOptions.Loaders`).
3. **render.Engine** executes `page.html` then wraps **layout.html** chain (root → leaf).
4. Template helpers: `tmplfunc` (`dict`, `slugify`, …) + **island** FuncMap (`island`, `islandRuntime`).
5. **ISR:** look up `cache.Key(method, path, locale)`; on HIT/STALE serve body with weak `ETag` and `X-Cache`; revalidate in background when stale; on MISS render and `Set`.

## Build path (SSG / ISR seed)

1. Optional Vite `npm run build` for islands → `public/islands`.
2. Discover routes; skip pure SSR.
3. Expand dynamic segments via `Paths` or **content collections** (`ContentPaths`).
4. Write `out/**/index.html`, seed ISR cache entries.
5. Emit `sitemap.xml` + `robots.txt` through `irmik/seo`.

## Packages

| Package | Role |
|---------|------|
| `irmik` | `App`, lifecycle, `AdaptLoader` |
| `irmik/router` | Discover + Gin handlers per mode |
| `irmik/render` | `html/template` engine |
| `irmik/island` | Vite manifest / dev URLs |
| `irmik/content` | Markdown + frontmatter |
| `irmik/seo` | Head tags, sitemap, robots |
| `irmik/cache` | memory / disk; Redis via `cache/redisx` |
| `irmik/db`, `db/sqlite|postgres|mysql`, `migrate`, `gormx` | SQL open (opt-in drivers) + migrations (+ optional GORM) |
| `irmik/validate` | Struct validation + Gin bind helpers |
| `irmik/session`, `session/redisx`, `csrf` | Cookie sessions (+ optional Redis) + CSRF |
| `irmik/auth`, `irmik/rbac` | Login/JWT/passwords/OAuth stubs + RBAC |
| `irmik/middleware` | Recovery, request ID, health, secure headers, rate limit |
| `irmik/sse`, `irmik/ws` | SSE + WebSocket hubs |
| `irmik/config` | `irmik.yaml` + env |
| `irmik/plugin` | before/after start/build/stop hooks |
| `irmik/tmplfunc`, `slug`, `fsutil` | shared helpers (StatiGo-inspired) |
| `internal/cli`, `internal/build`, `internal/hmr` | CLI plumbing |

## Config surface

Primary file: `irmik.yaml` (`app`, `server`, `cache`, `database`, `session`, `auth`, `build`, `content`, `seo`, `islands`, `i18n`). Env overrides: `IRMIK_ENV`, `IRMIK_BASE_URL`, `IRMIK_PORT`, `IRMIK_CACHE_DRIVER`, `REDIS_URL`, `DATABASE_URL`, `IRMIK_JWT_SECRET`, `IRMIK_SESSION_DRIVER`, …

## Non-goals (core)

Full i18n URL maps, minify-on-serve, and a DI container are out of scope for the thin core. Opt-in platform packages (upload, storage, queue, openapi, grpcx, observe, …) live beside the core — see [catalog.md](catalog.md). Auth: [auth.md](auth.md); database: [database.md](database.md); realtime: [realtime.md](realtime.md).
