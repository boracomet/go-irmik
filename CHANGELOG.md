# Changelog

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
