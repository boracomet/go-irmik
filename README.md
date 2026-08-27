<p align="center"><img src="assets/irmik.png" alt="Irmik" width="320" /></p>

# Irmik

[![CI](https://github.com/boracomet/go-irmik/actions/workflows/ci.yml/badge.svg)](https://github.com/boracomet/go-irmik/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/boracomet/go-irmik.svg)](https://pkg.go.dev/github.com/boracomet/go-irmik)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Keep Gin. Stop rebuilding the application layer.**

`app.Engine` is still `*gin.Engine`. Irmik is file routes, SSR/SSG/ISR, and
opt-in session/CSRF/RBAC/HTMX admin so you don't rebuild that glue every
Friday. JSON-only APIs should stay on Gin. The thin core at v0.2.0 is true
because heavy catalog (GORM, S3, OTel, gRPC, asynq) lives in nested modules —
this is not a megakit.

It is for:
- Admin and internal tools that need sessions, CSRF, RBAC, and HTMX.
- Server-rendered sites with file-based routes.

Current release: **[v0.2.0](CHANGELOG.md)**.

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

Core lives in the root module:

```sh
go get github.com/boracomet/go-irmik
```

That does **not** download AWS, GORM, the OpenTelemetry SDK, gRPC, or asynq.
SQL drivers, Redis, migrate, and similar opt-in packages that still live in
the root module are downloaded with it; they are linked only if you import them.

Heavy catalog backends are **nested modules**. Import paths are unchanged;
`go get` the nested path when you need one:

```sh
go get github.com/boracomet/go-irmik/irmik/db/gormx
go get github.com/boracomet/go-irmik/irmik/storage/s3x
go get github.com/boracomet/go-irmik/irmik/observe/otelx
go get github.com/boracomet/go-irmik/irmik/grpcx
go get github.com/boracomet/go-irmik/irmik/queue/asynqx
```

Nested modules are versioned with their own tags (for example
`irmik/db/gormx/v0.2.0`) once that cut is tagged. Until then, pin a commit or
`@main`. See [catalog](docs/catalog.md).

Links: [catalog](docs/catalog.md) · [auth](docs/auth.md) ·
[security](docs/security.md) · [devtools](docs/devtools.md) ·
[comparison](docs/compare.md) · [changelog](CHANGELOG.md) ·
[examples](examples)

## License

[MIT](LICENSE)
