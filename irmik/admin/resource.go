// Package admin provides thin HTMX CRUD conventions for admin UIs:
// session flash ↔ HX-Trigger helpers, pagination re-export patterns, and
// embeddable table/form/delete-confirm template snippets.
//
// Pair with irmik/htmx, irmik/csrf, irmik/validate, and irmik/paginate.
// See docs/admin.md and examples/admin.
package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/htmx"
	"github.com/boracomet/go-irmik/irmik/paginate"
)

// Resource describes naming conventions for a CRUD resource (for docs + helpers).
type Resource struct {
	Name       string // singular, e.g. "item"
	Plural     string // e.g. "items"
	BasePath   string // e.g. "/admin/items"
	Permission string // optional RBAC permission, e.g. "items:manage"
}

// ListQuery is an alias for paginate options tailored to admin tables.
func ListQuery(c *gin.Context, opts paginate.Options) paginate.Params {
	return paginate.Parse(c, opts)
}

// RenderOrRedirect renders an HTMX partial when HX-Request is set; otherwise
// redirects (full page navigation). Useful after create/update/delete.
func RenderOrRedirect(c *gin.Context, redirectURL string, render func(*gin.Context) error) {
	if htmx.IsRequest(c) {
		if render != nil {
			_ = render(c)
		}
		return
	}
	c.Redirect(http.StatusSeeOther, redirectURL)
}

// DeleteConfirmPath returns the conventional confirm-delete path for an id.
func (r Resource) DeleteConfirmPath(id string) string {
	base := r.BasePath
	if base == "" {
		base = "/admin/" + r.Plural
	}
	return base + "/" + id + "/delete"
}
