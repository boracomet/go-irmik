# StatiGo → Irmik: lessons learned

Source researched: [Elagoht/StatiGo](https://github.com/Elagoht/StatiGo) (cloned 2026-08-10).  
License: **MIT** (Copyright 2026 Elagoht) — ideas and small reimplementations are fine; attribute here; do not paste large verbatim chunks without keeping the MIT notice.

## What StatiGo is

StatiGo is a **static-first**, SEO-oriented Go site framework:

| Area | StatiGo | Irmik Phase 1 |
|------|---------|---------------|
| HTTP | **chi** (`net/http`) | **Gin** |
| Routes | JSON registry (`config/routes.json`) + i18n path maps | File-based `app/` + `_meta.yaml` |
| UI | `html/template` layouts/pages/partials | `html/template` + **React/Vite islands** |
| Content | No Markdown collections | `content/` + frontmatter (goldmark) |
| Cache | Memory + disk, Brotli, strategies | Memory / disk / redis + ISR TTL |
| Build | `prerender` warms cache via `httptest` | `irmik build` → `out/` HTML + sitemap |
| DX | `air` + Makefile | `irmik dev` + fsnotify / Vite |
| Deploy | Single binary + `embed.FS` | Optional later; not Phase 1 focus |

**Overlap with Phase 1 is moderate:** caching/ISR patterns, template FuncMap, prerender bootstrap, CLI cache commands, SEO helpers, graceful shutdown. **Low overlap:** Gin, file-based routing, Markdown content, islands. StatiGo is closer to a polished static CMS shell than a Next-like meta-framework.

## Architecture snapshot

```
main.go
  → godotenv + slog logger
  → embed FS (templates, static, translations, config)
  → i18n + router.Registry (canonical ↔ locale paths)
  → templates.Renderer (per-page Clone to avoid define conflicts)
  → cache.Manager (sync.Map memory + disk .html/.br)
  → chi router + middleware stack
  → LoadRoutesFromJSON → RegisterRoutes
  → optional CLI: prerender | clear-cache
```

Caching strategies on routes: `static` | `incremental` | `dynamic` | `immutable`.  
`incremental` ≈ soft ISR (stale after 24h or explicit MarkStale + background `httptest` revalidate).

## Adopt (Phase 1-relevant)

### 1. Template FuncMap helpers — **adopted**

StatiGo ships a useful set: `dict`, `set`, `add`/`sub`/`div`/`mod`, `until`, `slugify`, `safeHTML`/`safeURL`, `formatDate`/`formatDateTime`, `prettyJson`.

Irmik: package `irmik/tmplfunc` (merge into render via `Options.Funcs` when render lands). Reimplemented in Irmik style; Turkish-aware slug lives in `irmik/slug`.

### 2. Safe disk cache filenames — **adopted**

StatiGo maps cache keys to filenames by replacing `/` and `:` with `_`. Useful for `.irmik/cache` and SSR HTML on disk.

Irmik: `irmik/fsutil.SafeFileName` (also folds `|` used by Irmik cache keys).

### 3. EnsureDir / recursive HTML walk — **adopted**

Small FS helpers for build/export and template discovery without racing `irmik/render`.

Irmik: `irmik/fsutil`.

### 4. Prerender via in-process `httptest` — **defer (wire after build/CLI)**

Bootstrap warms pages by `ServeHTTP` on the real router with a worker pool (concurrency ≈ 10), `SetSync` for disk, skip `dynamic` and `{param}` routes unless expanded. Same idea maps cleanly to Irmik SSG/ISR seed: walk registered routes → render → write `out/` or cache.

### 5. ETag + `If-None-Match` → 304 — **defer (cache/ISR middleware)**

StatiGo serves HIT with weak ETag and returns 304 when matching. Good for ISR/static serve after parallel agents finish middleware wiring.

### 6. Cache strategies ↔ Irmik modes — **document mapping**

| StatiGo | Irmik |
|---------|--------|
| `dynamic` | `ssr` |
| `static` | `ssg` / `static` |
| `incremental` | `isr` (prefer explicit `revalidate` over hardcoded 24h) |
| `immutable` | hashed public assets / CSR shell that never revalidates |

Prefer Irmik’s TTL-based ISR over StatiGo’s fixed 24h incremental rule.

### 7. Dual disk write (HTML + compressed) — **defer / optional**

StatiGo writes `.html` + `.br` (Brotli). Irmik already has disk store; Brotli dual-write is an optimization for production serve, not required for Phase 1 correctness. Consider later behind config.

### 8. Page-template `Clone()` isolation — **defer to render agent**

Each page is `base.Clone()` + parse one page file so `{{define}}`/`{{block}}` do not collide. Important for layouts/partials; implement inside `irmik/render`, not here.

### 9. CLI: `prerender` / `clear-cache` aliases — **defer to CLI**

Useful UX: `irmik build` (primary) plus `irmik cache clear` (alias invalidate). StatiGo aliases (`bake`, `warm`) are cute but optional.

### 10. Static asset Cache-Control — **defer**

Dev: `no-cache`. Prod: long-lived `immutable` for hashed CSS/JS/fonts; short TTL for `sitemap.xml` / `robots.txt`. Apply when serving `public/` / `out/`.

### 11. Makefile + air for Go reload — **inspiration only**

Irmik’s first-class path is `irmik dev`. Document air as an optional fallback for pure-Go apps without Vite.

### 12. `embed.FS` single-binary deploy — **Phase 1.5 / 2**

Nice for docs/marketing sites. Phase 1 ships `out/` + runtime Gin; embed can be an example or later CLI flag.

## Skip / low value for Phase 1

| Item | Why skip |
|------|----------|
| JSON route registry + chi | Irmik is file-based + Gin |
| i18n URL prefixes as core | Config stub exists; full locale routing is later |
| Rate limit / honeypot / IP ban / security headers | Ops/security Phase 2+ |
| Brotli response compression middleware | Optional; CDN often owns this |
| Custom mini CLI framework | Use cobra in `internal/cli` |
| Webhook cache invalidation | Phase 2 |
| Minify-on-serve (tdewolff) | Prefer minify at `irmik build` time if needed |
| Price/currency/YouTube template helpers | Domain-specific; not meta-framework core |
| Hardcoded GTM_ID injection | Keep analytics out of core |

## Config patterns worth mirroring (conceptually)

StatiGo uses `.env` heavily. Irmik already prefers `irmik.yaml` + env overrides — keep that. Steal *names/ideas*, not the env-only approach:

- `BASE_URL` → already `app.baseURL` / `IRMIK_BASE_URL`
- `CACHE_DIR` → `cache.diskDir`
- `SHUTDOWN_TIMEOUT` → already on `server`
- Scheduled revalidation hour → optional later; ISR TTL is enough for Phase 1

## File / content conventions

StatiGo:

```text
templates/layouts|pages|partials
static/
config/routes.json
translations/{lang}.json
```

Irmik (keep plan conventions):

```text
app/**/page.html + _meta.yaml
templates/          # shared partials
content/<collection>/**/*.md
public/
islands/
```

No Markdown pipeline in StatiGo — do **not** copy content handling from it. Slugify + date helpers help blog templates fed by Irmik content collections.

## Gin notes

StatiGo has **no Gin**. Middleware patterns (response capture for cache, language context, layout data) translate to Gin middleware/`c.Writer` wrappers, but implement against Gin idioms when wiring ISR — do not port chi middleware literally.

## License notes

- StatiGo: **MIT** (Elagoht, 2026).
- Reimplemented packages under Irmik: original work inspired by StatiGo patterns; this doc is the attribution.
- If substantial verbatim code is ever copied, retain MIT copyright notice in that file header.

## Adopted in this pass (additive, non-conflicting)

| Package / file | Role |
|----------------|------|
| `docs/statigo-lessons.md` | This document |
| `irmik/fsutil` | `EnsureDir`, `SafeFileName`, HTML walk helpers |
| `irmik/slug` | Unicode + Turkish-aware URL slugify |
| `irmik/tmplfunc` | Shared `html/template` FuncMap for render to merge |

## Deferred until parallel agents finish

1. Wire `tmplfunc.Map()` into `irmik/render` `Options.Funcs`.
2. Use `fsutil.SafeFileName` in disk cache / SSG export paths if not already hashed.
3. Build-time prerender worker pool (`httptest` or direct Engine.Render) in `internal/build`.
4. ISR middleware: ETag / 304 / `X-Cache` HIT|MISS|STALE.
5. `irmik cache clear` CLI command.
6. Prod Cache-Control for static assets + islands hashed files.
7. Per-page template Clone isolation in render (if not already done).
8. Optional HTML minify at build (not request path).
9. Optional `embed.FS` example for single-binary docs sites.
10. Background eager revalidate with concurrency limit (StatiGo semaphore=10).
