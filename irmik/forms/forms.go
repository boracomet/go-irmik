// Package forms provides form parse/validate glue and CSRF field HTML helpers.
//
// It wraps irmik/validate for binding. CSRF helpers accept a token string so
// this package does not import irmik/session — wire csrf.Token(c) from the
// handler/consumer.
package forms

import (
	"html/template"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/validate"
)

// BindForm binds and validates form POST/PUT data into dst.
func BindForm(c *gin.Context, dst any) error {
	return validate.BindForm(c, dst)
}

// BindQuery binds and validates query parameters into dst.
func BindQuery(c *gin.Context, dst any) error {
	return validate.BindQuery(c, dst)
}

// BindJSON binds and validates JSON into dst.
func BindJSON(c *gin.Context, dst any) error {
	return validate.BindJSON(c, dst)
}

// Abort writes a 400 JSON validation error response.
func Abort(c *gin.Context, err error) {
	validate.Abort(c, err)
}

// FieldErrors extracts validate.Errors when present.
func FieldErrors(err error) (validate.Errors, bool) {
	return validate.AsErrors(err)
}

// CSRFFieldName is the default form field used by irmik/csrf.
const CSRFFieldName = "_csrf"

// CSRFInput returns a hidden input HTML snippet for the given token.
// Pass csrf.Token(c) from the request handler.
func CSRFInput(token string) template.HTML {
	return CSRFInputNamed(CSRFFieldName, token)
}

// CSRFInputNamed returns a hidden input with a custom field name.
func CSRFInputNamed(name, token string) template.HTML {
	if name == "" {
		name = CSRFFieldName
	}
	return template.HTML(`<input type="hidden" name="` +
		template.HTMLEscapeString(name) + `" value="` +
		template.HTMLEscapeString(token) + `">`)
}

// MustBindForm binds the form or aborts with 400. Returns false if aborted.
func MustBindForm(c *gin.Context, dst any) bool {
	if err := BindForm(c, dst); err != nil {
		Abort(c, err)
		return false
	}
	return true
}
