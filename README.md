<p align="center"><img src="assets/irmik.png" alt="Irmik" width="320" /></p>

# Irmik

[![CI](https://github.com/boracomet/go-irmik/actions/workflows/ci.yml/badge.svg)](https://github.com/boracomet/go-irmik/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/boracomet/go-irmik.svg)](https://pkg.go.dev/github.com/boracomet/go-irmik)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Irmik is a Go framework for Gin applications with server-rendered pages,
sessions, security helpers, and opt-in packages.

**Core promise:** file-based `app/` routes, SSR/SSG/ISR render modes, and
security-minded admin helpers (opt-in session/CSRF/RBAC/HTMX).

It is for:
- Admin and internal tools that need sessions, CSRF, RBAC, and a JSON API.
- Server-rendered sites with file-based routes.
- Teams that want optional database, upload, realtime, and queue packages.

**v0.1.1:** opt-in responsive images (`imagex.Pipeline`, upload variants) and a
development overlay (badge, errors, live reload with `irmik dev`). See the
[changelog](CHANGELOG.md).

## Quick start

```sh
go install github.com/boracomet/go-irmik/cmd/irmik@latest
irmik new hello
cd hello && go run .
```

See [examples/admin](examples/admin) for the complete admin and API example.

```go
cfg := config.Default()
app, err := irmik.New(cfg)
if err != nil { panic(err) }
app.Engine.GET("/", handler)
```

For production, set `IRMIK_JWT_SECRET` to a strong random value. Empty and demo
JWT secrets are rejected outside development. Do not store access tokens in
`localStorage`; use an httpOnly BFF cookie or a server session.

`go get` downloads this module’s full `go.mod` graph, including AWS, GORM, and
OpenTelemetry, even when your application never imports those packages.
**Binary linking is opt-in via import; module download is not.** Unused catalog
packages stay out of the binary only if you do not import them.

Links: [catalog](docs/catalog.md) · [auth](docs/auth.md) ·
[security](docs/security.md) · [devtools](docs/devtools.md) ·
[comparison](docs/compare.md) · [changelog](CHANGELOG.md) ·
[examples](examples)

## License

[MIT](LICENSE)
