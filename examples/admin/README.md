# Admin + API example

Product showcase for Irmik: **session/HTMX admin UI** + **JWT REST API** over SQLite.

```bash
cd examples/admin
go run .
# http://127.0.0.1:8080
```

Demo account (password `password123`):

- `admin@example.com` — role `admin`
- `editor@example.com` — role `editor`

## Admin UI

1. Open http://127.0.0.1:8080/login
2. Sign in → `/admin/items`
3. Create / edit / delete items (HTMX partials + CSRF)

Packages used: `irmik/admin`, `irmik/htmx`, `irmik/csrf`, `irmik/paginate`, `irmik/rbac`, `irmik/session`, `irmik/auth`.

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

### Next.js (App Router) sketch

CORS allows `http://localhost:3000` in this demo.

```ts
// lib/irmik.ts
const API = process.env.IRMIK_API_URL ?? "http://127.0.0.1:8080";

export async function login(email: string, password: string) {
  const res = await fetch(`${API}/api/v1/token`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  if (!res.ok) throw new Error("login failed");
  return res.json() as Promise<{ access_token: string; expires_at: string }>;
}

export async function listItems(token: string) {
  const res = await fetch(`${API}/api/v1/items`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!res.ok) throw new Error("list failed");
  return res.json();
}
```

Store the access token in an httpOnly cookie or server session — never expose long-lived secrets to the browser without XSS protections.

## Security wiring

- `EnableSecureDefaults()` — global rate limit
- `EnableSessions` + `EnableAuth` + CSRF on browser routes
- `middleware.LoginRateLimit` on `/login` and `/api/v1/token`
- RBAC `items:manage` on `/admin/*`
- JWT `RequireJWT` on `/api/v1/items*`
- SQLite via blank-import `irmik/db/sqlite` (in-memory by default)
