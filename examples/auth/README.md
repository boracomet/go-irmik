# Auth example

Minimal demo: cookie sessions, CSRF, password login, JWT, RBAC.

```bash
cd examples/auth
go run .
# http://127.0.0.1:8080
```

Demo accounts (password `password123`):

- `admin@example.com` — role `admin` (can `GET /admin`)
- `editor@example.com` — role `editor` (dashboard only)

```bash
# session login (cookie + CSRF header from GET /)
curl -c jar -b jar http://127.0.0.1:8080/
curl -c jar -b jar -H "Content-Type: application/json" -H "X-CSRF-Token: <csrf>" \
  -d '{"email":"admin@example.com","password":"password123"}' \
  http://127.0.0.1:8080/login

# JWT
curl -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"password123"}' \
  http://127.0.0.1:8080/token
```
