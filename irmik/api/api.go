// Package api provides thin REST helpers: JSON responses, a standard error
// envelope, and an /api/v1 group mount. Opt-in — not wired by irmik.New.
//
// Error envelope:
//
//	{ "error": { "code": "not_found", "message": "item not found", "details": … } }
//
// Pair with irmik/auth for JWT: mount authenticator.RequireJWT() (or MiddlewareJWT)
// on the v1 group / routes. See docs/api.md.
package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/validate"
)

// ErrorBody is the nested object under "error".
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// ErrorResponse is the standard JSON error envelope.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// JSON writes a JSON response with the given status.
func JSON(c *gin.Context, status int, v any) {
	if status <= 0 {
		status = http.StatusOK
	}
	c.JSON(status, v)
}

// Error writes the standard error envelope without aborting the chain.
// details is optional: pass at most one value (extra values are ignored).
func Error(c *gin.Context, status int, code, message string, details ...any) {
	if status <= 0 {
		status = http.StatusInternalServerError
	}
	body := ErrorResponse{Error: ErrorBody{Code: code, Message: message}}
	if len(details) > 0 && details[0] != nil {
		body.Error.Details = details[0]
	}
	c.JSON(status, body)
}

// Abort is Error followed by c.Abort().
func Abort(c *gin.Context, status int, code, message string, details ...any) {
	Error(c, status, code, message, details...)
	c.Abort()
}

// V1 returns the /api/v1 router group on engine.
func V1(engine *gin.Engine) *gin.RouterGroup {
	if engine == nil {
		return nil
	}
	return engine.Group("/api/v1")
}

// MountV1 creates /api/v1 under r and optionally runs mount funcs against it.
// r may be *gin.Engine or a parent *gin.RouterGroup (anything implementing gin.IRouter).
func MountV1(r gin.IRouter, mount ...func(*gin.RouterGroup)) *gin.RouterGroup {
	if r == nil {
		return nil
	}
	g := r.Group("/api/v1")
	for _, fn := range mount {
		if fn != nil {
			fn(g)
		}
	}
	return g
}

// BindJSON binds JSON into dst and validates it (irmik/validate).
func BindJSON(c *gin.Context, dst any) error {
	return validate.BindJSON(c, dst)
}

// BindQuery binds query params into dst and validates them.
func BindQuery(c *gin.Context, dst any) error {
	return validate.BindQuery(c, dst)
}

// AbortValidation writes a 400 error envelope for validation/bind failures and aborts.
func AbortValidation(c *gin.Context, err error) {
	if err == nil {
		return
	}
	if ve, ok := validate.AsErrors(err); ok {
		Abort(c, http.StatusBadRequest, "validation_failed", "validation failed", ve)
		return
	}
	Abort(c, http.StatusBadRequest, "bad_request", err.Error())
}

// Common status helpers (optional convenience).

// NotFound aborts with 404 not_found.
func NotFound(c *gin.Context, message string) {
	if message == "" {
		message = "not found"
	}
	Abort(c, http.StatusNotFound, "not_found", message)
}

// Unauthorized aborts with 401 unauthorized.
func Unauthorized(c *gin.Context, message string) {
	if message == "" {
		message = "unauthorized"
	}
	Abort(c, http.StatusUnauthorized, "unauthorized", message)
}

// Forbidden aborts with 403 forbidden.
func Forbidden(c *gin.Context, message string) {
	if message == "" {
		message = "forbidden"
	}
	Abort(c, http.StatusForbidden, "forbidden", message)
}

// Internal logs diagnostic detail and sends a generic 500 response. Do not pass
// its message through to clients: handlers may supply database or filesystem errors.
func Internal(c *gin.Context, message string) {
	if message != "" {
		slog.Error("internal API error", "error", message)
	}
	Abort(c, http.StatusInternalServerError, "internal_error", "internal error")
}
