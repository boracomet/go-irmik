# Changelog

## Unreleased

## v0.2.0 - 2026-08-27

Honesty and correctness pass on v0.1.1. Catalog is frozen; no new packages.

- **Nested catalog modules** — `gormx`, `s3x`, `otelx`, `grpcx`, and `asynqx` are separate Go modules so `go get github.com/boracomet/go-irmik` no longer downloads AWS, GORM, the OTel SDK, gRPC, or asynq. Import paths are unchanged; `go get` the nested path. Repo `go.work` keeps local tests working.
- **WS hub** — `ServeHTTP` starts the event loop if `Start`/`Run` was skipped, so a forgotten `Start()` cannot deadlock the HTTP goroutine. Register/unregister do not hang after `Close`.
- **`EnableSecureHeaders`** replaces the config mounted in `New` instead of stacking a second middleware, so `Skip` flags actually drop defaults.
- **golangci-lint** — explicit small v2 set: govet, errcheck, staticcheck, unused, ineffassign, gofmt, goimports.
- **SSR template cache** — `render.Engine` compiles each page and layout once and reuses the parsed tree. `Reload` (and `SetFuncs` / `SetIslandFunc`) drop the cache so disk edits from `irmik dev` still apply.
- **ISR revalidate** runs the same loader path as the request (cloned GET + params) instead of skipping `*gin.Context` loaders.
- **SSE** clears the per-connection write deadline so `http.Server.WriteTimeout` does not kill streams; `App` sets `IdleTimeout` (default 60s).
- Router **500s** return a generic body and log the error with request id.
- **`irmik new`** pins `github.com/boracomet/go-irmik v0.2.0` (no sibling `replace`) and does not double-register `/health`.
- JWT refresh/revoke uses a `RefreshStore` (default process-local memory with TTL/GC). Not a multi-replica store.
- OAuth `GitHubStub` / `GoogleStub` replace the production-looking providers; `Exchange` always returns `ErrOAuthNotImplemented`.
- `Context.MustUser` panics when no user is present.
- `cache.New` errors on unknown drivers (same as session/queue).
- `queue.Memory` delayed-enqueue vs `Close` no longer panics on a closed channel.
- **HEAD** does not write a body; ISR cache keys are shared with GET.
- Plugin `after_start` / stop hook errors are logged. Production JWT secrets must be ≥32 characters (denylist kept).
- Docs: binary linking is opt-in via import; module download is not.
- Leftover: tiny `go.opentelemetry.io/otel/trace` and `otelhttp` may still appear as **indirect** root deps via golang-migrate; that is not the OTel SDK.

## v0.1.1

Responsive images and a development overlay. Production behavior is unchanged:
the overlay is not mounted outside `development`.

- **`irmik/imagex` pipeline** — opt-in `Pipeline` serves `/_irmik/img` and an
  `{{ img }}` helper that emits `srcset` (375 / 768 / 1440, WebP). Local files
  only; remote URLs and arbitrary widths are rejected. Hero images can set
  `priority` so they are not lazy-loaded.
- **Upload variants** — `Variants` / `WriteVariants` encode allowlisted widths
  at save time (`name-375.webp`, `name-1440.webp`) for admin media.
- **Dev overlay** — in development, HTML pages get a bottom-left Irmik badge.
  The panel lists template and window errors, file routes, and listen address.
  `irmik dev` reloads the browser after `app/` or `templates/` saves. Island
  compile errors stay in the Vite overlay.
- Docs: catalog, architecture, and this changelog.

## v0.1.0

This release establishes the first tagged Irmik baseline. CI now runs race tests,
linting, and vulnerability checks on main. Production applications reject empty
and known demo JWT secrets before they listen. WebSocket, CORS, proxy, upload,
and Markdown defaults now fail closed or require an explicit unsafe option.
JWT access tokens include a `jti`, and the auth package supports rotating
refresh tokens and user-level refresh revocation. `MiddlewareJWT` treats a
present invalid token as unauthorized. The CLI adds `irmik new` for a small
local starter. The admin example includes a Next.js BFF pattern that keeps
tokens out of browser JavaScript. Framework defaults now bind to localhost,
sanitize request IDs, and minimize readiness responses. The README is short,
English-only, and points to focused documentation.
