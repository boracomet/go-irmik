// Package router discovers file-based app/ routes and binds them to Gin
// with SSR / SSG / ISR / Static / CSR handlers.
package router

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	"github.com/boracomet/go-irmik/irmik/cache"
	"github.com/boracomet/go-irmik/irmik/meta"
	"github.com/boracomet/go-irmik/irmik/middleware"
	"github.com/boracomet/go-irmik/irmik/render"
)

// Loader supplies per-request (or build-time) data for a route.
// Keyed by Gin path pattern (e.g. "/blog/:slug").
//
// ISR background revalidation clones the original GET (path, query, headers,
// params) onto a detached gin.Context and calls the same Loader. Loaders must
// be safe to re-run that way: do not depend on the live ResponseWriter, and
// treat missing session/auth gin keys as anonymous. Per-user ISR is unsupported.
type Loader func(c *gin.Context) (any, error)

// Route is a discovered page route under app/.
type Route struct {
	// URLPath is the Gin route pattern, e.g. /blog/:slug.
	URLPath string
	// RelDir is the directory under app/ ("" for root), using slash separators.
	RelDir string
	// PageFile is the absolute path to page.html.
	PageFile string
	// LayoutFiles are absolute paths to layout.html from root → leaf.
	LayoutFiles []string
	// MetaFile is the absolute path to _meta.yaml if present.
	MetaFile string
	Meta     meta.PageMeta
}

// metaFile is the on-disk YAML shape for _meta.yaml.
type metaFile struct {
	Mode       string `yaml:"mode"`
	Revalidate string `yaml:"revalidate"`
	Canonical  string `yaml:"canonical"`
	Robots     string `yaml:"robots"`
	Sitemap    *bool  `yaml:"sitemap"`
	NoIndex    bool   `yaml:"noIndex"`
}

// FilePathToGin converts an app-relative directory path to a Gin URL pattern.
// Dynamic segments: [slug] → :slug, [...slug] → *slug.
func FilePathToGin(relDir string) string {
	relDir = filepath.ToSlash(strings.Trim(relDir, "/"))
	if relDir == "" || relDir == "." {
		return "/"
	}
	parts := strings.Split(relDir, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		// Skip Next-style private folders and route groups: _foo, (group)
		if strings.HasPrefix(p, "_") {
			continue
		}
		if strings.HasPrefix(p, "(") && strings.HasSuffix(p, ")") {
			continue
		}
		if strings.HasPrefix(p, "[") && strings.HasSuffix(p, "]") {
			inner := p[1 : len(p)-1]
			if strings.HasPrefix(inner, "...") {
				out = append(out, "*"+strings.TrimPrefix(inner, "..."))
			} else {
				out = append(out, ":"+inner)
			}
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return "/"
	}
	return "/" + strings.Join(out, "/")
}

// Discover walks appDir for page.html files and returns routes.
func Discover(appDir string) ([]Route, error) {
	appDir = filepath.Clean(appDir)
	info, err := os.Stat(appDir)
	if err != nil {
		return nil, fmt.Errorf("router: app dir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("router: app dir is not a directory: %s", appDir)
	}

	var routes []Route
	err = filepath.WalkDir(appDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == "node_modules" || base == ".git" || strings.HasPrefix(base, "_") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "page.html" {
			return nil
		}
		dir := filepath.Dir(path)
		rel, err := filepath.Rel(appDir, dir)
		if err != nil {
			return err
		}
		if rel == "." {
			rel = ""
		}
		urlPath := FilePathToGin(rel)
		metaPath := filepath.Join(dir, "_meta.yaml")
		pageMeta := meta.Default()
		metaFilePath := ""
		if _, err := os.Stat(metaPath); err == nil {
			m, err := loadMeta(metaPath)
			if err != nil {
				return fmt.Errorf("router: meta %s: %w", metaPath, err)
			}
			pageMeta = m
			metaFilePath = metaPath
		}
		layouts, err := collectLayouts(appDir, dir)
		if err != nil {
			return err
		}
		routes = append(routes, Route{
			URLPath:     urlPath,
			RelDir:      filepath.ToSlash(rel),
			PageFile:    path,
			LayoutFiles: layouts,
			MetaFile:    metaFilePath,
			Meta:        pageMeta,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return routes, nil
}

func loadMeta(path string) (meta.PageMeta, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return meta.PageMeta{}, err
	}
	var mf metaFile
	if err := yaml.Unmarshal(b, &mf); err != nil {
		return meta.PageMeta{}, err
	}
	pageMeta := meta.Default()
	if mf.Mode != "" {
		pageMeta.Mode = meta.Mode(strings.ToLower(strings.TrimSpace(mf.Mode)))
	}
	if mf.Revalidate != "" {
		d, err := time.ParseDuration(mf.Revalidate)
		if err != nil {
			return meta.PageMeta{}, fmt.Errorf("revalidate: %w", err)
		}
		pageMeta.Revalidate = d
	}
	pageMeta.Canonical = mf.Canonical
	pageMeta.Robots = mf.Robots
	pageMeta.NoIndex = mf.NoIndex
	if mf.Sitemap != nil {
		pageMeta.Sitemap = *mf.Sitemap
	}
	return pageMeta, nil
}

// collectLayouts returns layout.html paths from app root down to pageDir (root first).
func collectLayouts(appDir, pageDir string) ([]string, error) {
	appDir = filepath.Clean(appDir)
	pageDir = filepath.Clean(pageDir)

	var dirs []string
	cur := pageDir
	for {
		dirs = append(dirs, cur)
		if cur == appDir {
			break
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	// dirs are leaf → root; reverse to root → leaf
	for i, j := 0, len(dirs)-1; i < j; i, j = i+1, j-1 {
		dirs[i], dirs[j] = dirs[j], dirs[i]
	}

	var layouts []string
	for _, d := range dirs {
		lp := filepath.Join(d, "layout.html")
		if st, err := os.Stat(lp); err == nil && !st.IsDir() {
			layouts = append(layouts, lp)
		}
	}
	return layouts, nil
}

// Options configures router mounting.
type Options struct {
	AppDir    string
	OutDir    string
	PublicDir string
	Locale    string
	Cache     cache.Store
	Renderer  *render.Engine
	Loaders   map[string]Loader
	StaticFS  http.FileSystem // optional override for public/
}

// Router holds discovered routes and serves them.
type Router struct {
	opts     Options
	routes   []Route
	mu       sync.Mutex
	inflight map[string]bool // ISR revalidate keys
}

// New discovers routes but does not bind Gin yet.
func New(opts Options) (*Router, error) {
	if opts.AppDir == "" {
		opts.AppDir = "app"
	}
	if opts.Locale == "" {
		opts.Locale = "en"
	}
	if opts.Loaders == nil {
		opts.Loaders = map[string]Loader{}
	}
	routes, err := Discover(opts.AppDir)
	if err != nil {
		return nil, err
	}
	return &Router{
		opts:     opts,
		routes:   routes,
		inflight: map[string]bool{},
	}, nil
}

// Routes returns discovered routes.
func (r *Router) Routes() []Route {
	return append([]Route(nil), r.routes...)
}

// RegisterLoader attaches a data loader for a Gin path pattern.
func (r *Router) RegisterLoader(pattern string, loader Loader) {
	if r.opts.Loaders == nil {
		r.opts.Loaders = map[string]Loader{}
	}
	r.opts.Loaders[pattern] = loader
}

// Mount registers GET handlers for all discovered routes on engine.
func (r *Router) Mount(engine *gin.Engine) {
	// Serve public/ and out/ static assets when configured.
	if r.opts.PublicDir != "" {
		if st, err := os.Stat(r.opts.PublicDir); err == nil && st.IsDir() {
			engine.StaticFS("/public", gin.Dir(r.opts.PublicDir, false))
			// Also map root-level static files via NoRoute fallback is handled per-mode.
		}
	}

	for i := range r.routes {
		rt := r.routes[i]
		h := r.handler(rt)
		engine.GET(rt.URLPath, h)
		engine.HEAD(rt.URLPath, h)
	}
}

func (r *Router) handler(rt Route) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch rt.Meta.Mode {
		case meta.ModeStatic:
			r.serveStatic(c, rt)
		case meta.ModeSSG:
			r.serveSSG(c, rt)
		case meta.ModeISR:
			r.serveISR(c, rt)
		case meta.ModeCSR:
			r.serveCSR(c, rt)
		default: // SSR
			r.serveSSR(c, rt)
		}
	}
}

func (r *Router) serveSSR(c *gin.Context, rt Route) {
	body, err := r.renderRoute(c, rt)
	if err != nil {
		renderFail(c, err)
		return
	}
	writeHTML(c, http.StatusOK, "text/html; charset=utf-8", body)
}

func (r *Router) serveCSR(c *gin.Context, rt Route) {
	// Prefer page.html if present (discovered routes always have page.html);
	// render normally — CSR pages typically are thin shells with islands.
	body, err := r.renderRoute(c, rt)
	if err != nil {
		renderFail(c, err)
		return
	}
	writeHTML(c, http.StatusOK, "text/html; charset=utf-8", body)
}

func (r *Router) serveStatic(c *gin.Context, rt Route) {
	if path := r.outFileFor(rt, c); path != "" {
		if raw, err := os.ReadFile(path); err == nil {
			c.Data(http.StatusOK, "text/html; charset=utf-8", raw)
			return
		}
	}
	// Fallback: public mirror or live render.
	if r.opts.PublicDir != "" {
		cand := filepath.Join(r.opts.PublicDir, strings.TrimPrefix(staticRel(rt, c), "/"))
		if raw, err := os.ReadFile(cand); err == nil {
			c.Data(http.StatusOK, "text/html; charset=utf-8", raw)
			return
		}
	}
	r.serveSSR(c, rt)
}

func (r *Router) serveSSG(c *gin.Context, rt Route) {
	if path := r.outFileFor(rt, c); path != "" {
		if raw, err := os.ReadFile(path); err == nil {
			c.Data(http.StatusOK, "text/html; charset=utf-8", raw)
			return
		}
	}
	// Fallback to live render when out/ missing (e.g. before first build).
	r.serveSSR(c, rt)
}

func (r *Router) serveISR(c *gin.Context, rt Route) {
	if r.opts.Cache == nil {
		r.serveSSR(c, rt)
		return
	}
	key := cache.Key(http.MethodGet, c.Request.URL.Path, r.opts.Locale)
	ctx := c.Request.Context()
	entry, err := r.opts.Cache.Get(ctx, key)
	switch {
	case err == nil && !entry.Expired():
		status := "HIT"
		if entry.Stale() {
			status = "STALE"
			r.revalidateAsync(key, rt, c)
		}
		writeCachedHTML(c, entry.Body, contentType(entry), status)
		return
	case err == nil && entry.Expired():
		// Fall through to render; optionally still serve stale briefly.
		if len(entry.Body) > 0 {
			writeCachedHTML(c, entry.Body, contentType(entry), "STALE")
			r.revalidateAsync(key, rt, c)
			return
		}
	case errors.Is(err, cache.ErrMiss):
		// miss
	default:
		if err != nil && !errors.Is(err, cache.ErrMiss) {
			// cache error: degrade to SSR
			r.serveSSR(c, rt)
			return
		}
	}

	body, err := r.renderRoute(c, rt)
	if err != nil {
		renderFail(c, err)
		return
	}
	_ = r.opts.Cache.Set(ctx, key, r.isrEntry(body, rt.Meta.Revalidate))
	writeCachedHTML(c, body, "text/html; charset=utf-8", "MISS")
}

// writeCachedHTML sets weak ETag + X-Cache and honors If-None-Match → 304.
func writeCachedHTML(c *gin.Context, body []byte, ct, cacheStatus string) {
	etag := weakETag(body)
	c.Header("ETag", etag)
	c.Header("X-Cache", cacheStatus)
	if noneMatch := c.GetHeader("If-None-Match"); noneMatch != "" && noneMatch == etag {
		c.Status(http.StatusNotModified)
		return
	}
	writeHTML(c, http.StatusOK, ct, body)
}

func writeHTML(c *gin.Context, status int, ct string, body []byte) {
	if ct == "" {
		ct = "text/html; charset=utf-8"
	}
	c.Header("Content-Type", ct)
	c.Header("Content-Length", strconv.Itoa(len(body)))
	if c.Request.Method == http.MethodHead {
		c.Status(status)
		return
	}
	c.Data(status, ct, body)
}

func renderFail(c *gin.Context, err error) {
	rid := middleware.GetRequestID(c)
	slog.Error("irmik: render error",
		"err", err,
		"path", c.Request.URL.Path,
		"request_id", rid,
	)
	c.Header("Content-Type", "text/plain; charset=utf-8")
	if rid != "" {
		c.Header("X-Request-ID", rid)
	}
	c.String(http.StatusInternalServerError, "internal server error")
}

func weakETag(body []byte) string {
	sum := sha256.Sum256(body)
	return fmt.Sprintf(`W/"%x"`, sum[:8])
}

func (r *Router) isrEntry(body []byte, revalidate time.Duration) cache.Entry {
	if revalidate <= 0 {
		revalidate = 60 * time.Second
	}
	now := time.Now()
	return cache.Entry{
		Body:        bytes.Clone(body),
		ContentType: "text/html; charset=utf-8",
		StaleAt:     now.Add(revalidate),
		ExpiresAt:   now.Add(revalidate * 2), // soft expiry after 2× TTL
	}
}

func (r *Router) revalidateAsync(key string, rt Route, src *gin.Context) {
	r.mu.Lock()
	if r.inflight[key] {
		r.mu.Unlock()
		return
	}
	r.inflight[key] = true
	r.mu.Unlock()

	req := cloneRequestForRevalidate(src)
	params := append(gin.Params(nil), src.Params...)

	go func() {
		defer func() {
			r.mu.Lock()
			delete(r.inflight, key)
			r.mu.Unlock()
		}()
		gc, _ := gin.CreateTestContext(httptest.NewRecorder())
		gc.Request = req
		gc.Params = params
		body, err := r.renderRoute(gc, rt)
		if err != nil {
			slog.Error("irmik: isr revalidate", "err", err, "path", req.URL.Path)
			return
		}
		_ = r.opts.Cache.Set(req.Context(), key, r.isrEntry(body, rt.Meta.Revalidate))
	}()
}

// cloneRequestForRevalidate copies the live GET so background ISR uses the same
// loader path (URL, query, headers, params) without the original ResponseWriter
// or a cancelled client context.
func cloneRequestForRevalidate(c *gin.Context) *http.Request {
	req := c.Request.Clone(context.Background())
	req.Method = http.MethodGet
	return req
}

func (r *Router) renderRoute(c *gin.Context, rt Route) ([]byte, error) {
	if r.opts.Renderer == nil {
		return nil, fmt.Errorf("router: renderer is nil")
	}
	var data any
	if loader, ok := r.opts.Loaders[rt.URLPath]; ok && loader != nil {
		var err error
		data, err = loader(c)
		if err != nil {
			return nil, err
		}
	}
	params := map[string]string{}
	for _, p := range c.Params {
		params[p.Key] = p.Value
	}
	return r.opts.Renderer.RenderToBytes(rt.PageFile, rt.LayoutFiles, render.Data{
		Meta:   rt.Meta,
		Data:   data,
		Params: params,
		Path:   c.Request.URL.Path,
	})
}

func (r *Router) outFileFor(rt Route, c *gin.Context) string {
	if r.opts.OutDir == "" {
		return ""
	}
	rel := staticRel(rt, c)
	// Prefer path/index.html for directories.
	candidates := []string{
		filepath.Join(r.opts.OutDir, strings.TrimPrefix(rel, "/")),
	}
	if !strings.HasSuffix(rel, ".html") {
		candidates = append(candidates,
			filepath.Join(r.opts.OutDir, strings.TrimPrefix(rel, "/"), "index.html"),
			filepath.Join(r.opts.OutDir, strings.TrimPrefix(rel, "/")+".html"),
		)
	}
	for _, cand := range candidates {
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand
		}
	}
	return ""
}

func staticRel(rt Route, c *gin.Context) string {
	path := c.Request.URL.Path
	if path == "" || path == "/" {
		return "/index.html"
	}
	if strings.Contains(rt.URLPath, ":") || strings.Contains(rt.URLPath, "*") {
		return path
	}
	return path
}

func contentType(e cache.Entry) string {
	if e.ContentType != "" {
		return e.ContentType
	}
	return "text/html; charset=utf-8"
}
