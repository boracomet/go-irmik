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
| `irmik/cache` | memory / disk / redis |
| `irmik/config` | `irmik.yaml` + env |
| `irmik/plugin` | before/after start/build/stop hooks |
| `irmik/tmplfunc`, `slug`, `fsutil` | shared helpers (StatiGo-inspired) |
| `internal/cli`, `internal/build`, `internal/hmr` | CLI plumbing |

## Config surface

Primary file: `irmik.yaml` (`app`, `server`, `cache`, `build`, `content`, `seo`, `islands`, `i18n`). Env overrides: `IRMIK_ENV`, `IRMIK_BASE_URL`, `IRMIK_PORT`, `IRMIK_CACHE_DRIVER`, `REDIS_URL`, …

## Non-goals (Phase 1)

Auth, database, queues, OpenAPI, gRPC, full i18n URL maps, Brotli dual-write, minify-on-serve — deferred to later phases.
