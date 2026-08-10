// Package htmx — quick reference for admin handlers:
//
//	if htmx.IsRequest(c) {
//	    htmx.Retarget(c, "#flash")
//	    htmx.Trigger(c, "toast")
//	    _ = htmx.RenderPartial(c, rowTmpl, "row", data)
//	    return
//	}
//	htmx.Redirect(c, "/users")
package htmx
