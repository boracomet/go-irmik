// Package render provides an html/template engine with layouts, partials,
// and an island helper stub for the Vite/React island package to replace.
package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/boracomet/go-irmik/irmik/tmplfunc"
)

// IslandFunc renders an interactive island placeholder.
// The island package should replace the default via Engine.SetIslandFunc.
type IslandFunc func(name string, props any) (template.HTML, error)

// Options configures the template engine.
type Options struct {
	// AppDir is the file-based routes root (layouts live here).
	AppDir string
	// TemplatesDir holds shared partials (optional).
	TemplatesDir string
	// Funcs are extra template functions merged into the FuncMap.
	Funcs template.FuncMap
}

// Engine compiles and executes page + layout templates.
//
// Page and layout files are parsed once and reused until Reload, SetFuncs, or
// SetIslandFunc. Shared partials are Clone'd per file so {{define}}/{{block}}
// names in one page cannot collide with another.
type Engine struct {
	mu         sync.RWMutex
	opts       Options
	funcs      template.FuncMap
	island     IslandFunc
	partials   *template.Template // shared partials base; never executed
	compiled   map[string]*template.Template
	compileGen uint64 // bumped when compiled is dropped
}

// New creates an Engine. Call Reload after construction (or Mount) to load partials.
func New(opts Options) (*Engine, error) {
	if opts.AppDir == "" {
		opts.AppDir = "app"
	}
	e := &Engine{
		opts:     opts,
		island:   DefaultIsland,
		funcs:    template.FuncMap{},
		compiled: make(map[string]*template.Template),
	}
	// Merge shared template helpers (dict, slugify, formatDate, …).
	for k, v := range tmplfunc.Map() {
		e.funcs[k] = v
	}
	if opts.Funcs != nil {
		for k, v := range opts.Funcs {
			e.funcs[k] = v
		}
	}
	e.rebuildFuncs()
	if err := e.Reload(); err != nil {
		return nil, err
	}
	return e, nil
}

// SetIslandFunc replaces the {{ island }} helper implementation.
func (e *Engine) SetIslandFunc(f IslandFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if f == nil {
		f = DefaultIsland
	}
	e.island = f
	e.rebuildFuncs()
	// The default island helper looks up e.island at execute time, but dropping
	// compiled trees keeps the FuncMap snapshot on cached templates consistent.
	e.dropCompiledLocked()
}

// SetFuncs merges additional FuncMap entries (e.g. SEO helpers).
func (e *Engine) SetFuncs(fm template.FuncMap) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for k, v := range fm {
		e.funcs[k] = v
	}
	e.rebuildFuncs()
	e.dropCompiledLocked()
}

func (e *Engine) rebuildFuncs() {
	island := e.island
	if island == nil {
		island = DefaultIsland
	}
	base := template.FuncMap{
		"island": func(name string, props ...any) (template.HTML, error) {
			var p any
			if len(props) > 0 {
				p = props[0]
			}
			e.mu.RLock()
			f := e.island
			e.mu.RUnlock()
			if f == nil {
				f = DefaultIsland
			}
			return f(name, p)
		},
		// TODO(seo): title, meta, jsonld helpers plug in via SetFuncs
	}
	for k, v := range e.funcs {
		if k == "island" {
			continue
		}
		base[k] = v
	}
	e.funcs = base
	_ = island
}

func (e *Engine) dropCompiledLocked() {
	e.compiled = make(map[string]*template.Template)
	e.compileGen++
}

// Reload reloads shared partials from TemplatesDir and drops compiled
// page/layout templates so the next Render reads files from disk again.
func (e *Engine) Reload() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	t := template.New("partials").Funcs(e.funcs)
	dir := e.opts.TemplatesDir
	if dir != "" {
		if err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".html") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			name := filepath.ToSlash(strings.TrimPrefix(path, dir))
			name = strings.TrimPrefix(name, "/")
			if _, err := t.New(name).Parse(string(b)); err != nil {
				return fmt.Errorf("parse partial %s: %w", path, err)
			}
			return nil
		}); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	e.partials = t
	e.dropCompiledLocked()
	return nil
}

// Data is the standard view model passed to templates.
type Data struct {
	Meta    any
	Data    any
	Params  map[string]string
	Path    string
	Content template.HTML
}

// Render writes the page wrapped by layouts (innermost → outermost).
// pageFile is an absolute or project-relative path to page.html.
// layoutFiles are ordered root → leaf (root layout is outermost).
//
// Templates are compiled on first use and served from cache. Disk changes are
// not visible until Reload (or SetFuncs / SetIslandFunc, which drop the cache).
func (e *Engine) Render(w io.Writer, pageFile string, layoutFiles []string, data Data) error {
	pt, err := e.getCompiled(pageFile, "page")
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := pt.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute page %s: %w", pageFile, err)
	}
	inner := buf.Bytes()

	// Wrap with layouts from leaf → root (closest layout first).
	for i := len(layoutFiles) - 1; i >= 0; i-- {
		lf := layoutFiles[i]
		lt, err := e.getCompiled(lf, "layout")
		if err != nil {
			return err
		}
		wrapData := data
		wrapData.Content = template.HTML(inner)
		buf.Reset()
		if err := lt.Execute(&buf, wrapData); err != nil {
			return fmt.Errorf("execute layout %s: %w", lf, err)
		}
		inner = bytes.Clone(buf.Bytes())
	}

	_, err = w.Write(inner)
	return err
}

// RenderToBytes is a convenience wrapper around Render.
func (e *Engine) RenderToBytes(pageFile string, layoutFiles []string, data Data) ([]byte, error) {
	var buf bytes.Buffer
	if err := e.Render(&buf, pageFile, layoutFiles, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (e *Engine) getCompiled(path, kind string) (*template.Template, error) {
	key := kind + "\x00" + path

	e.mu.RLock()
	if t, ok := e.compiled[key]; ok {
		e.mu.RUnlock()
		return t, nil
	}
	funcs := cloneFuncs(e.funcs)
	partials := e.partials
	gen := e.compileGen
	e.mu.RUnlock()

	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s %s: %w", kind, path, err)
	}
	t, err := parseWithPartials(kind, string(body), funcs, partials)
	if err != nil {
		return nil, fmt.Errorf("parse %s %s: %w", kind, path, err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.compileGen == gen {
		if cached, ok := e.compiled[key]; ok {
			return cached, nil
		}
		e.compiled[key] = t
	}
	return t, nil
}

// parseWithPartials builds an isolated template set for one page or layout file.
// Clone of the shared partials base is required: parsing {{define}}/{{block}}
// into the shared set would let later files overwrite earlier names.
func parseWithPartials(name, src string, funcs template.FuncMap, partials *template.Template) (*template.Template, error) {
	var t *template.Template
	if partials != nil {
		c, err := partials.Clone()
		if err != nil {
			return nil, err
		}
		t = c.New(name).Funcs(funcs)
	} else {
		t = template.New(name).Funcs(funcs)
	}
	return t.Parse(src)
}

// DefaultIsland emits a hydrate target for the React/Vite island runtime.
// TODO(island): replace via Engine.SetIslandFunc with manifest-aware scripts.
func DefaultIsland(name string, props any) (template.HTML, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("island: empty name")
	}
	propsJSON := "null"
	if props != nil {
		b, err := json.Marshal(props)
		if err != nil {
			return "", fmt.Errorf("island props: %w", err)
		}
		propsJSON = string(b)
	}
	out := fmt.Sprintf(
		`<div data-island="%s" data-props="%s"></div>`,
		html.EscapeString(name),
		html.EscapeString(propsJSON),
	)
	return template.HTML(out), nil
}

// CSRShell is a minimal HTML shell used for ModeCSR when no page content is needed.
const CSRShell = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{with .Meta}}{{.Canonical}}{{else}}App{{end}}</title>
  {{/* TODO(island): inject Vite/client or prod manifest scripts */}}
</head>
<body>
  <div id="root">{{.Content}}</div>
  {{island "app" .Data}}
</body>
</html>
`

func cloneFuncs(in template.FuncMap) template.FuncMap {
	out := make(template.FuncMap, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
