// Package devtools injects a development-only overlay (badge, errors, live reload).
//
// Mounted automatically by irmik.New when cfg.IsDev(). Production binaries
// still link this package; routes and HTML inject are not registered unless
// Attach is called. Do not enable outside development.
package devtools

import (
	"embed"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

//go:embed logo.png overlay.js
var overlayFS embed.FS

// Snapshot is the JSON payload for /_irmik/dev/info.
type Snapshot struct {
	Env        string      `json:"env"`
	Addr       string      `json:"addr"`
	LiveReload bool        `json:"liveReload"`
	Routes     []RouteInfo `json:"routes"`
}

// RouteInfo is one file route shown in the overlay.
type RouteInfo struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
}

// Options configures the overlay.
type Options struct {
	// Info is called for GET /_irmik/dev/info. Optional.
	Info func() Snapshot
}

// Dev is the in-process overlay hub.
type Dev struct {
	opts Options
	mu   sync.RWMutex
	subs map[chan event]struct{}
}

type event struct {
	name string
	data string
}

// New constructs a hub. Call Inject before other middleware, then Mount.
func New(opts Options) *Dev {
	return &Dev{opts: opts, subs: map[chan event]struct{}{}}
}

// Attach registers inject middleware and /_irmik/dev/* routes.
func Attach(engine *gin.Engine, opts Options) *Dev {
	d := New(opts)
	engine.Use(d.Inject())
	d.Mount(engine)
	return d
}

// Mount registers overlay assets and the SSE stream.
func (d *Dev) Mount(engine gin.IRoutes) {
	engine.GET("/_irmik/dev/logo.png", d.logo)
	engine.GET("/_irmik/dev/overlay.js", d.script)
	engine.GET("/_irmik/dev/info", d.info)
	engine.GET("/_irmik/dev/events", d.events)
}

func (d *Dev) logo(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	data, err := overlayFS.ReadFile("logo.png")
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Data(http.StatusOK, "image/png", data)
}

func (d *Dev) script(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	data, err := overlayFS.ReadFile("overlay.js")
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Data(http.StatusOK, "text/javascript; charset=utf-8", data)
}

func (d *Dev) info(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, d.snapshot())
}

func (d *Dev) snapshot() Snapshot {
	if d.opts.Info == nil {
		return Snapshot{Env: "development", LiveReload: true, Routes: []RouteInfo{}}
	}
	s := d.opts.Info()
	if s.Routes == nil {
		s.Routes = []RouteInfo{}
	}
	s.LiveReload = true
	return s
}

func (d *Dev) events(c *gin.Context) {
	if rc := http.NewResponseController(c.Writer); rc != nil {
		_ = rc.SetWriteDeadline(time.Time{})
	}
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.Status(http.StatusInternalServerError)
		return
	}
	h := c.Writer.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	flusher.Flush()

	ch := d.subscribe()
	defer d.unsubscribe(ch)

	ctx := c.Request.Context()
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()

	writeEvent(c.Writer, flusher, "info", mustJSON(d.snapshot()))

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			_, _ = io.WriteString(c.Writer, ": ping\n\n")
			flusher.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			writeEvent(c.Writer, flusher, ev.name, ev.data)
		}
	}
}

func writeEvent(w io.Writer, flusher http.Flusher, name, data string) {
	_, _ = io.WriteString(w, "event: "+name+"\n")
	for _, line := range strings.Split(data, "\n") {
		_, _ = io.WriteString(w, "data: "+line+"\n")
	}
	_, _ = io.WriteString(w, "\n")
	flusher.Flush()
}

func (d *Dev) subscribe() chan event {
	ch := make(chan event, 16)
	d.mu.Lock()
	d.subs[ch] = struct{}{}
	d.mu.Unlock()
	return ch
}

func (d *Dev) unsubscribe(ch chan event) {
	d.mu.Lock()
	if _, ok := d.subs[ch]; ok {
		delete(d.subs, ch)
		close(ch)
	}
	d.mu.Unlock()
}

func (d *Dev) broadcast(name, data string) {
	ev := event{name: name, data: data}
	d.mu.RLock()
	defer d.mu.RUnlock()
	for ch := range d.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// Reload tells connected browsers to location.reload() after a successful
// template/route refresh.
func (d *Dev) Reload(path string) {
	d.broadcast("reload", mustJSON(map[string]string{"path": path}))
}

// Report pushes a server/template error to the overlay without reloading.
func (d *Dev) Report(source, message string) {
	d.broadcast("problem", mustJSON(map[string]string{
		"source":  source,
		"message": message,
	}))
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// snippet is inserted before </body> (or appended).
func snippet() string {
	return `<script src="/_irmik/dev/overlay.js" defer></script>`
}

func wrapPlainError(msg string) string {
	return "<!doctype html><meta charset=utf-8><title>Irmik error</title><pre>" +
		html.EscapeString(msg) + "</pre>"
}
