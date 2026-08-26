# Contributing to Irmik

Thanks for helping improve Irmik. The core promise is **file-based routes, render modes, and security-minded admin helpers**, with extra capabilities behind explicit imports — please keep that shape when proposing changes. The catalog is frozen: do not add new packages until the module graph work lands.

## Before you start

1. Skim [docs/roadmap.md](docs/roadmap.md) (especially **Explicitly not for core**) and [docs/catalog.md](docs/catalog.md).
2. Prefer **opt-in packages** under `irmik/<name>/` over wiring new deps into `irmik.New`.
3. Heavy backends (Redis, S3, OTel, gRPC, DB drivers) should stay behind blank-imports or explicit `Open`/`New` APIs.

## Development

```bash
git clone https://github.com/boracomet/go-irmik.git
cd go-irmik
go test ./...
```

Useful demos:

| Example | Command |
|---------|---------|
| Admin + JWT API | `cd examples/admin && go run .` |
| Auth | `cd examples/auth && go run .` |
| Realtime | `cd examples/realtime && go run .` |
| Blog / islands | `cd examples/blog && go run .` |

CLI: `go run ./cmd/irmik --help`.

## Pull requests

- Keep PRs focused; one concern per PR when practical.
- Match existing package style and naming.
- Add or update tests for behavior changes.
- Update docs when you change public APIs or product paths (`README.md`, `docs/*`, example READMEs).
- Do not commit secrets, `.env` files with real credentials, or large unrelated binaries.

## Issues

- Use a clear title and steps to reproduce.
- Include Go version, OS, and whether the issue is in core vs an opt-in package.
- **Never** paste production secrets, JWT keys, session cookies, or private DSNs into issues or PRs. Redact them.

Security vulnerabilities: see [SECURITY.md](SECURITY.md).

## Code of conduct (short)

Be respectful. Assume good intent. Harassment or abuse is not welcome.
