# Admin UI helpers (`irmik/admin`, `irmik/paginate`)

Opt-in packages for SSR + HTMX admin panels. Pair with `irmik/htmx`, `irmik/csrf`, `irmik/session`, `irmik/validate`, and `irmik/rbac` ([rbac.md](rbac.md)).

## Pagination (`irmik/paginate`)

```go
p := paginate.Parse(c, paginate.Options{
    DefaultPerPage: 20,
    MaxPerPage:     100,
    DefaultSort:    "created_at",
    DefaultOrder:   "desc",
    SortWhitelist:  []string{"id", "title", "created_at"},
})
// SQL-friendly:
offset, limit := p.Offset(), p.Limit()
orderBy := p.OrderBy(map[string]string{
    "title":      "items.title",
    "created_at": "items.created_at",
})
```

Query params: `page`, `per_page`, `sort`, `order` (`asc`|`desc`), `q`.  
Sort/filter columns must come from a **whitelist** — never interpolate raw query values into SQL identifiers. `FilterColumn` helps for dynamic WHERE columns.

## Flash ↔ HTMX

```go
admin.SetFlash(c, admin.FlashSuccess, "Saved")
admin.FlashHX(c, admin.FlashSuccess, "Saved") // session flash + HX-Trigger JSON
flash := admin.FlashData(c)                   // map for templates
```

`FlashHX` emits `HX-Trigger` like `{"admin:flash":{"level":"success","message":"Saved"}}` when `HX-Request` is present.

## CRUD template snippets

Embedded under `irmik/admin/templates` (`list.html`, `form.html`, `confirm_delete.html`, `flash.html`):

```go
tmpl, _ := admin.ParseTemplates(nil)
// or admin.TemplatesFS() to copy/customize
```

Conventions (see example):

| Partial | Role |
|---------|------|
| Table list | Search `q`, row edit/delete links, HTMX `hx-get` into `#irmik-admin-main` |
| Form | CSRF field, field errors, `hx-post` |
| Delete confirm | POST confirm + cancel |

```go
admin.RenderOrRedirect(c, "/admin/items", func(c *gin.Context) error {
    return htmx.RenderPartial(c, tmpl, "items_list_partial", data)
})
```

## Suggested wiring

```go
app.EnableSecureDefaults()
_ = app.EnableSessions()
a := app.EnableAuth()
app.Engine.Use(a.InjectSessionUser(lookup))
app.Engine.Use(csrf.Middleware(csrf.Options{}))

adminUI := app.Engine.Group("/admin")
adminUI.Use(a.RequireAuthSession(), rbac.RequirePermission(reg, rbac.Perm("items", "read")))
// tmpl := template.New("x").Funcs(rbac.FuncMap(reg)).ParseFS(...)
// {{if Can .User "items:delete"}}…{{end}}
```

Full working demo: **[examples/admin](../examples/admin)** (Items CRUD + JWT `/api/v1/items`).
