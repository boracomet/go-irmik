# Roadmap — what else without overloading

Irmik’s rule stays: **thin core, wide opt-in catalog**. New work should be import-only packages or small CLI/config helpers — never hard-wired into `irmik.New`.

## Recommended next (prioritized)

### P1 — low cost, high value (opt-in)

| Idea | Why | Shape |
|------|-----|--------|
| **Full cron + timezone scheduler** | Current `scheduler` is a 5-field subset | Extend `irmik/scheduler` (opt-in) |
| **Audit middleware presets** | Admin apps need “who did what” fast | Thin wrappers on `irmik/audit` + Gin middleware |
| **Request logging helper** | Structured access logs without DIY slog glue | `observe` middleware preset |
| **CORS helper** | Every admin SPA / islands app needs it | Small `middleware` or `cors` package |
| **Health dependency checks** | `/ready` that pings DB/Redis/queue | Extend existing health/ready hooks |
| **HTMX helpers for admin** | Partial/HX response helpers, CSRF-friendly forms | Opt-in `irmik/htmx` — not a frontend framework |

### P2 — useful, still lean

| Idea | Why | Shape |
|------|-----|--------|
| **`embed.FS` static / single-binary sites** | Docs/marketing deploys (StatiGo lesson) | Example + optional build flag |
| **i18n routing** | Locale path prefixes / hreflang | Opt-in; keep out of core routing |
| **OpenAPI from Gin routes (codegen)** | Less hand-written OpenAPI | Experimental generator on `openapi` |
| **Richer Redis/asynq DX** | Queue already has asynqx | Docs + helpers, not core |
| **WebP / image pipeline polish** | Admin uploads | Keep in `imagex` |

## Explicitly **not** for core

Do **not** pull these into `irmik.New` or the root package:

- Heavy DI containers / service locators
- GraphQL-in-core (if ever: separate opt-in package)
- Full CMS / admin UI generator as a mandatory dependency
- Bundled Redis / AWS / OTel / gRPC by default
- Beego/GoFrame-style “everything linked” megakit
- Domain-specific template helpers (currency, YouTube embeds, …)

## North-star checks before adding anything

1. Can it stay behind an explicit import?
2. Does it help **admin/internal** or **SSR sites** without forcing both?
3. Does it preserve **Gin compatibility** and lean linking?

See [catalog.md](catalog.md) for what already exists and [compare.md](compare.md) for positioning.
