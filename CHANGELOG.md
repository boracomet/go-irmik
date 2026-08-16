# Changelog

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
