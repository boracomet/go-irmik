# Admin + API example

Product showcase for Irmik: **session/HTMX admin UI** + **JWT REST API** over SQLite.

```bash
cd examples/admin
go run .
# http://127.0.0.1:8080
```

Demo accounts (password `password123`):

| Email | Role | Can |
|-------|------|-----|
| `admin@example.com` | `admin` | read / write / **delete** items |
| `editor@example.com` | `editor` | read / write items (no delete) |

Roles and permissions are seeded into SQLite via `irmik/rbac/store` on startup, then loaded into `rbac.Registry`.

## Admin UI

1. Open http://127.0.0.1:8080/login
2. Sign in → `/admin/items`
3. Create / edit items (HTMX partials + CSRF)
4. As **admin**, Delete is visible; as **editor**, the Delete button is hidden (`Can` in templates) and `/admin/items/:id/delete` returns 403

Packages used: `irmik/admin`, `irmik/htmx`, `irmik/csrf`, `irmik/paginate`, `irmik/rbac`, `irmik/rbac/store`, `irmik/session`, `irmik/auth`.

RBAC docs: [docs/rbac.md](../../docs/rbac.md).

## REST API (`/api/v1`)

JWT for external clients (Next.js, mobile, etc.). Error envelope:

```json
{ "error": { "code": "not_found", "message": "item not found" } }
```

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| `POST` | `/api/v1/token` | — | body `{ "email", "password" }` → Bearer token |
| `GET` | `/api/v1/items` | Bearer | `?page&per_page&sort&order&q` |
| `POST` | `/api/v1/items` | Bearer | `{ "title", "body" }` |
| `GET` | `/api/v1/items/:id` | Bearer | |
| `PUT`/`PATCH` | `/api/v1/items/:id` | Bearer | |
| `DELETE` | `/api/v1/items/:id` | Bearer | |

```bash
# issue token
TOKEN=$(curl -s -X POST http://127.0.0.1:8080/api/v1/token \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"password123"}' \
  | jq -r .access_token)

# list / create
curl -s -H "Authorization: Bearer $TOKEN" 'http://127.0.0.1:8080/api/v1/items'
curl -s -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"title":"From API","body":"hello"}' \
  http://127.0.0.1:8080/api/v1/items
```

### Next.js BFF

See [`next`](next). Its server route handlers call Irmik and store the access
token in an `httpOnly` cookie. Client components fetch only `/api/items`; they
never see a Bearer token or the Irmik URL. Browser-to-Irmik CORS is only needed
without this BFF pattern.

## Security wiring

- `EnableSecureDefaults()` — global rate limit
- `EnableSessions` + `EnableAuth` + CSRF on browser routes
- `middleware.LoginRateLimit` on `/login` and `/api/v1/token`
- RBAC: `items:read` on `/admin/*`; `items:write` / `items:delete` on mutating routes (same on API)
- Template `Can` hides New / Edit / Delete when the role lacks the permission
- JWT `RequireJWT` on `/api/v1/items*`
- SQLite via blank-import `irmik/db/sqlite` (in-memory by default); RBAC tables co-located
- Refresh tokens are process-local (in-memory `RefreshStore`) — restarting this example invalidates them

**Demo-only:** JWT secret falls back to `dev-only-change-me-jwt-secret-32b` and session `Secure` is forced off for local HTTP. For real deploys set `IRMIK_JWT_SECRET`, enable Secure cookies, and use HTTPS — see [docs/security.md](../../docs/security.md).
