# REST API helpers (`irmik/api`)

Thin opt-in helpers for JSON APIs. Not wired by `irmik.New`.

## Error envelope

All helpers write:

```json
{
  "error": {
    "code": "not_found",
    "message": "item not found",
    "details": { "optional": true }
  }
}
```

| Helper | Behavior |
|--------|----------|
| `api.JSON(c, status, v)` | Success / any JSON body |
| `api.Error(c, status, code, msg, details?)` | Envelope, does not abort |
| `api.Abort(...)` | Envelope + `c.Abort()` |
| `api.NotFound` / `Unauthorized` / `Forbidden` / `Internal` | Common status shortcuts |
| `api.BindJSON` / `BindQuery` | validate glue |
| `api.AbortValidation` | `400` + `validation_failed` + field `details` |

## `/api/v1` group

```go
v1 := api.V1(app.Engine)
// or
api.MountV1(app.Engine, func(v1 *gin.RouterGroup) {
    v1.GET("/ping", func(c *gin.Context) { api.JSON(c, 200, gin.H{"ok": true}) })
})
```

## JWT with `irmik/auth`

```go
a := app.EnableAuth()

api.MountV1(app.Engine, func(v1 *gin.RouterGroup) {
    v1.POST("/token", issueTokenHandler) // public

    secured := v1.Group("")
    secured.Use(a.RequireJWT())
    secured.GET("/items", listItems)
})
```

Clients send `Authorization: Bearer <access_token>`. Issue tokens with `a.IssueAccessToken(user)`.

For browser cookie sessions use `irmik/csrf` + `RequireAuthSession` instead — keep JWT routes CSRF-free.

Status codes: `400` validation, `401` auth, `403` RBAC, `404` missing, `500` unexpected.

See **[examples/admin](../examples/admin)** for a full Items API.
