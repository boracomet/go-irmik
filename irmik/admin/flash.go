package admin

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/htmx"
	"github.com/boracomet/go-irmik/irmik/session"
)

// Common flash keys for admin UIs.
const (
	FlashNotice  = "notice"
	FlashError   = "error"
	FlashSuccess = "success"
)

// SetFlash stores a one-shot message for the next request (session flash).
func SetFlash(c *gin.Context, key, message string) {
	if c == nil || key == "" {
		return
	}
	if s := session.Get(c); s != nil {
		s.SetFlash(key, message)
	}
}

// PopFlash returns and clears an incoming flash string for this request.
func PopFlash(c *gin.Context, key string) string {
	if c == nil {
		return ""
	}
	s := session.Get(c)
	if s == nil {
		return ""
	}
	v, ok := s.PopFlash(key)
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

// FlashData collects common flash keys for template rendering.
func FlashData(c *gin.Context) map[string]string {
	out := map[string]string{}
	for _, k := range []string{FlashSuccess, FlashNotice, FlashError} {
		if m := PopFlash(c, k); m != "" {
			out[k] = m
		}
	}
	return out
}

// FlashHX sets session flash and, for HTMX requests, HX-Trigger with a JSON
// payload so the client can show a toast without a full redirect.
// Event name defaults to "admin:flash".
//
// Example trigger body: {"admin:flash":{"level":"success","message":"Saved"}}
func FlashHX(c *gin.Context, level, message string) {
	FlashHXEvent(c, "admin:flash", level, message)
}

// FlashHXEvent is FlashHX with a custom HTMX event name.
func FlashHXEvent(c *gin.Context, event, level, message string) {
	if level == "" {
		level = FlashNotice
	}
	SetFlash(c, level, message)
	if c == nil || !htmx.IsRequest(c) || event == "" {
		return
	}
	payload := map[string]any{
		event: map[string]string{
			"level":   level,
			"message": message,
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		htmx.Trigger(c, event)
		return
	}
	htmx.Trigger(c, string(b))
}
