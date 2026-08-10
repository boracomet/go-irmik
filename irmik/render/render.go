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
type Engine struct {
	mu       sync.RWMutex
	opts     Options
	funcs    template.FuncMap
	island   IslandFunc
	partials *template.Template // shared partials base
}

// New creates an Engine. Call Reload after construction (or Mount) to load partials.
func New(opts Options) (*Engine, error) {
	if opts.AppDir == "" {
		opts.AppDir = "app"
	}
	e := &Engine{
		opts:   opts,
		island: DefaultIsland,
		funcs:  template.FuncMap{},
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
}

// SetFuncs merges additional FuncMap entries (e.g. SEO helpers).
func (e *Engine) SetFuncs(fm template.FuncMap) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for k, v := range fm {
		e.funcs[k] = v
	}
	e.rebuildFuncs()
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

// Reload reloads shared partials from TemplatesDir.
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
func (e *Engine) Render(w io.Writer, pageFile string, layoutFiles []string, data Data) error {
	e.mu.RLock()
	funcs := cloneFuncs(e.funcs)
	partials := e.partials
	e.mu.RUnlock()

	pageBody, err := os.ReadFile(pageFile)
	if err != nil {
		return fmt.Errorf("read page %s: %w", pageFile, err)
	}

	// Render page content first.
	pt := template.New("page").Funcs(funcs)
	if partials != nil {
		for _, t := range partials.Templates() {
			if t.Name() == "partials" {
				continue
			}
			if _, err := pt.AddParseTree(t.Name(), t.Tree); err != nil {
				return err
			}
		}
	}
	if _, err := pt.Parse(string(pageBody)); err != nil {
		return fmt.Errorf("parse page %s: %w", pageFile, err)
	}

	var buf bytes.Buffer
	if err := pt.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute page %s: %w", pageFile, err)
	}
	inner := buf.Bytes()

	// Wrap with layouts from leaf → root (closest layout first).
	for i := len(layoutFiles) - 1; i >= 0; i-- {
		lf := layoutFiles[i]
		lb, err := os.ReadFile(lf)
		if err != nil {
			return fmt.Errorf("read layout %s: %w", lf, err)
		}
		lt := template.New("layout").Funcs(funcs)
		if partials != nil {
			for _, t := range partials.Templates() {
				if t.Name() == "partials" {
					continue
				}
				if _, err := lt.AddParseTree(t.Name(), t.Tree); err != nil {
					return err
				}
			}
		}
		if _, err := lt.Parse(string(lb)); err != nil {
			return fmt.Errorf("parse layout %s: %w", lf, err)
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
