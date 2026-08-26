# Irmik vs other Go stacks

Fair, high-level comparison. “Yes” means first-class or documented in-tree helpers; “DIY” means you assemble yourself; “Partial” means related features exist but not the same product shape as Irmik.

| | **Irmik** | **Gin alone** | **[StatiGo](https://github.com/Elagoht/StatiGo)** | **Echo** | **Buffalo** | **Fiber** |
|---|-----------|---------------|--------------------------------------------------|----------|-------------|-----------|
| **HTTP base** | Gin | Gin | chi (`net/http`) | Echo | gorilla/mux | fasthttp |
| **SSR / SSG / ISR** | Yes — route `_meta.yaml` modes + `irmik build` | No | Yes — static-first cache strategies (`dynamic` / `static` / `incremental`) | No | Templates / asset pipeline; not Next-like ISR modes | No |
| **Auth / RBAC** | Opt-in (`session`, `csrf`, `auth`, `rbac`, JWT) | DIY | Not a full auth/RBAC product (site security focus) | DIY / community | Sessions + generators (ecosystem) | DIY / middleware |
| **SQL / migrate** | Opt-in `db` + drivers + `migrate` (+ optional GORM) | DIY | No first-class SQL stack | DIY | Pop + Soda (batteries) | DIY |
| **Realtime** | Opt-in SSE + WebSocket hub | DIY | Not a focus | DIY | Background workers; WS DIY | WS middleware available |
| **Admin security defaults** | Headers by default; `EnableSecureDefaults` rate limit | None | Strong site defaults (headers, rate limit, honeypot, IP ban) | DIY | Convention defaults | DIY |
| **Opt-in catalog** | Wide — upload, queue, mail, openapi, storage, … ([catalog](catalog.md)) | N/A | Site-oriented kit (i18n, SEO, embed) | Middleware ecosystem | Full ecosystem (heavier) | Middleware ecosystem |
| **Weight** | Import-opt-in binaries; `go get` still pulls the full module graph (AWS/GORM/OTel in `go.mod`) | Very lean | Lean; single-binary `embed.FS` focus | Lean | Batteries-included | Lean-ish (fasthttp trade-offs) |
| **Best fit** | Admin/internal apps **and** SSR/SSG sites on Gin | APIs / custom glue | Content/SEO marketing & docs sites | APIs / microservices | Rapid traditional full-stack apps | High-throughput HTTP APIs |

## Positioning in one line each

| Stack | One-liner |
|-------|-----------|
| **Irmik** | Gin meta-framework: file routes + render modes + security-minded admin helpers + a frozen opt-in catalog. |
| **Gin** | Fast HTTP router/middleware; you own the rest. |
| **StatiGo** | Static-first SEO site framework on chi (overlap with Irmik’s cache/ISR *ideas*; different HTTP + routing model). |
| **Echo** | Thin, productive HTTP framework — similar “router first” niche to Gin. |
| **Buffalo** | Rails-like Go ecosystem (CLI, Pop, Plush, webpack) — opinionated full stack, not lean-link. |
| **Fiber** | Express-inspired API framework on fasthttp — speed-oriented, not SSR-mode oriented. |

## Adjacent tools

- **PocketBase** is a single binary with built-in data and an admin UI; Irmik does not provide either data model.
- **Ent** is schema-driven code generation; Irmik supports SQL and migrations but is not an ORM generator.
- **chi + templ** is a small, idiomatic composition for teams that want to assemble their own stack; Irmik has more conventions.
- **Next.js BFF** keeps browser cookies and UI at the edge while Irmik provides an API. Islands are not a replacement for Next App Router.

## What Irmik deliberately is *not*

- Not a CMS.
- Not a heavy DI / all-in-one enterprise kernel (contrast GoFrame/Beego-style megakit).
- Not GraphQL-as-built-in (REST + JWT helpers are first-class; GraphQL would be a separate opt-in if ever added).
- Not a replacement for pure API micro-frameworks when you only need routing.

See also [statigo-lessons.md](statigo-lessons.md), [catalog.md](catalog.md), [roadmap.md](roadmap.md).
