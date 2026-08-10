# Roadmap — what else without overloading

Irmik’s rule stays: **thin core, wide opt-in catalog**. New work should be import-only packages or small CLI/config helpers — never hard-wired into `irmik.New`.

## Product must-haves — done

| Idea | Why | Shape | Status |
|------|-----|--------|--------|
| **REST conventions** | Standard JSON errors + `/api/v1` for Next.js clients | `irmik/api` (`JSON`, `Error`, `Abort`, `V1` / `MountV1`) | Done |
| **Admin pagination / sort** | Safe list queries for tables + APIs | `irmik/paginate` (clamp + OrderBy whitelist) | Done |
| **HTMX CRUD skeleton** | Reusable list/form/delete + flash↔HX | `irmik/admin` + embedded `templates/` | Done |
| **Admin + API showcase** | One path to ship an internal product | `examples/admin` (session UI + JWT Items API) | Done |

Docs: [api.md](api.md), [admin.md](admin.md). Catalog: [catalog.md](catalog.md).

## P1 — low cost, high value (opt-in) — done

| Idea | Why | Shape | Status |
|------|-----|--------|--------|
| **Full cron + timezone scheduler** | Interval + timezone-aware cron | `irmik/scheduler` (`Every`, `AddCron` / `AddCronTZ`, robfig/cron/v3) | Done |
| **Audit middleware presets** | Admin apps need “who did what” fast | `irmik/audit.Middleware` → existing Sink | Done |
| **Request logging helper** | Structured access logs without DIY slog glue | `middleware.RequestLog` / `app.UseRequestLog()` | Done |
| **CORS helper** | Every admin SPA / islands app needs it | `irmik/cors` lean middleware | Done |
| **Health dependency checks** | `/ready` that pings DB/Redis/queue | `irmik/health` + `app.RegisterReadyCheck` | Done |
| **HTMX helpers for admin** | Partial/HX response helpers | Opt-in `irmik/htmx` | Done |

## P2 — useful, still lean

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
