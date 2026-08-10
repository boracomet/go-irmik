// Package htmx provides small Gin helpers for HTMX admin UIs.
//
// Detect HX-Request, set response headers (HX-Redirect, HX-Trigger, …),
// and optionally render an html/template partial. Opt-in — not a frontend framework.
package htmx

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	HeaderRequest     = "HX-Request"
	HeaderRedirect    = "HX-Redirect"
	HeaderTrigger     = "HX-Trigger"
	HeaderTriggerAfterSwap = "HX-Trigger-After-Swap"
	HeaderTriggerAfterSettle = "HX-Trigger-After-Settle"
	HeaderRetarget    = "HX-Retarget"
	HeaderReswap      = "HX-Reswap"
	HeaderPushURL     = "HX-Push-Url"
)

// IsRequest reports whether the client sent HX-Request: true.
func IsRequest(c *gin.Context) bool {
	if c == nil {
		return false
	}
	v := c.GetHeader(HeaderRequest)
	return v == "true" || v == "1"
}

// Redirect sets HX-Redirect so HTMX performs a client-side redirect.
func Redirect(c *gin.Context, url string) {
	c.Header(HeaderRedirect, url)
}

// Trigger sets HX-Trigger to a simple event name (e.g. "userUpdated").
// For JSON payloads, pass a JSON string yourself.
func Trigger(c *gin.Context, event string) {
	c.Header(HeaderTrigger, event)
}

// TriggerAfterSwap sets HX-Trigger-After-Swap.
func TriggerAfterSwap(c *gin.Context, event string) {
	c.Header(HeaderTriggerAfterSwap, event)
}

// TriggerAfterSettle sets HX-Trigger-After-Settle.
func TriggerAfterSettle(c *gin.Context, event string) {
	c.Header(HeaderTriggerAfterSettle, event)
}

// Retarget sets HX-Retarget (CSS selector for the swap target).
func Retarget(c *gin.Context, selector string) {
	c.Header(HeaderRetarget, selector)
}

// Reswap sets HX-Reswap (e.g. "outerHTML", "none").
func Reswap(c *gin.Context, strategy string) {
	c.Header(HeaderReswap, strategy)
}

// PushURL sets HX-Push-Url.
func PushURL(c *gin.Context, url string) {
	c.Header(HeaderPushURL, url)
}

// RenderPartial executes an html/template and writes HTML to the response.
// name may be empty to execute the root template. status defaults to 200 when 0.
func RenderPartial(c *gin.Context, tmpl *template.Template, name string, data any) error {
	return RenderPartialStatus(c, http.StatusOK, tmpl, name, data)
}

// RenderPartialStatus is RenderPartial with an explicit status code.
func RenderPartialStatus(c *gin.Context, status int, tmpl *template.Template, name string, data any) error {
	if tmpl == nil {
		return fmt.Errorf("htmx: nil template")
	}
	if status <= 0 {
		status = http.StatusOK
	}
	c.Status(status)
	c.Header("Content-Type", "text/html; charset=utf-8")
	if name == "" {
		return tmpl.Execute(c.Writer, data)
	}
	return tmpl.ExecuteTemplate(c.Writer, name, data)
}
