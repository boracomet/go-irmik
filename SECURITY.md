# Security Policy

## Supported versions

Security fixes are applied on the **latest `main`** of this repository. There is no long-term support branch yet.

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

Instead:

1. Use GitHub’s **[Private vulnerability reporting](https://github.com/boracomet/go-irmik/security/advisories/new)** if available, **or**
2. Email the maintainer via the contact listed on the [GitHub profile](https://github.com/boracomet) with a clear subject like `go-irmik security`.

Include:

- Affected package / example path
- Impact and a minimal reproduction (no production secrets)
- Suggested fix if you have one

You should receive an acknowledgement when practical. Please allow time for investigation before public disclosure.

## What not to include

Do **not** paste into issues, PRs, or advisories:

- Live JWT secrets, session secrets, API keys, or passwords
- Production database URLs with credentials
- Customer or user personal data

Use placeholders (`REDACTED`, `dev-only-…`) instead.

## Hardening checklist (apps built on Irmik)

Operators should:

1. Set a strong `IRMIK_JWT_SECRET` (never deploy demo defaults); non-development apps reject empty and known demo JWT secrets at startup.
2. Enable Secure session cookies in production (`session.secure` / non-development default)
3. Call `app.EnableSecureDefaults()` for admin UIs and use CSRF on cookie/session forms
4. Serve over HTTPS; configure trusted proxies and WebSocket allowed origins
5. Blank-import only the drivers you need

See [docs/security.md](docs/security.md) and [docs/auth.md](docs/auth.md).
