// Package island wires React/Vite islands into html/template pages.
//
// # Template FuncMap
//
// Register Manager.FuncMap() on your template set (alongside other helpers):
//
//	funcs := islandMgr.FuncMap()
//	t := template.New("").Funcs(funcs)
//
// Then in templates:
//
//	{{ island "Counter" (dict "initial" 0) }}
//
// which emits a mount node plus script (and CSS) tags:
//
//	<div data-island="Counter" data-props='{"initial":0}'></div>
//	<link rel="stylesheet" href="/islands/assets/...css">   <!-- prod, if any -->
//	<script type="module" src="..."></script>
//
// Optional helpers:
//
//	{{ islandRuntime }}  — Vite HMR client in development (@vite/client)
//	{{ islandScripts "Counter" "ThemeToggle" }} — scripts/CSS only (no mounts)
//
// # Dev vs prod
//
// Dev (Options.Dev): script src = {DevServer}/islands/{Name}.tsx
// (and islandRuntime → {DevServer}/@vite/client).
//
// Prod: reads Vite manifest.json from OutDir (see LoadManifestFromOutDir) and
// emits /{PublicPath}/{chunk.file} (default PublicPath "islands" → /islands/...).
//
// # Expected app layout
//
//	islands/
//	  _hydrate.tsx     # shared createIsland helper (not a Vite entry)
//	  Counter.tsx      # default export + createIsland("Counter", Counter)
//	  ThemeToggle.tsx
//	vite.config.ts     # multi-entry over islands/*.{tsx,jsx}, manifest: true
//	public/islands/    # Vite outDir + manifest.json
//
// See templates/README.md and templates/scaffold/ for a starter Vite config
// and client entry pattern.
package island

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"path"
	"strings"
	"sync"

	"github.com/boracomet/go-irmik/irmik/config"
)

// Options configures island rendering.
type Options struct {
	Enabled bool
	// Dev selects the Vite dev server instead of the production manifest.
	Dev bool
	// DevServer is the Vite origin, e.g. "http://localhost:5173".
	DevServer string
	// Dir is the islands source directory name used in dev script URLs (default "islands").
	Dir string
	// OutDir is where Vite writes assets + manifest (default "public/islands").
	OutDir string
	// PublicPath is the URL path prefix for built assets (default "islands").
	// Scripts become /{PublicPath}/{file} in production.
	PublicPath string
	// Ext is the source extension used for dev URLs (default ".tsx").
	Ext string
}

// OptionsFromConfig maps config.IslandsConfig into Options.
func OptionsFromConfig(cfg config.IslandsConfig, isDev bool) Options {
	return Options{
		Enabled:    cfg.Enabled,
		Dev:        isDev,
		DevServer:  cfg.DevServer,
		Dir:        cfg.Dir,
		OutDir:     cfg.OutDir,
		PublicPath: "islands",
		Ext:        ".tsx",
	}
}

// Manager renders island mount points and associated Vite assets.
type Manager struct {
	opts     Options
	manifest Manifest

	mu sync.RWMutex
}

// New constructs a Manager. In production (non-Dev) with Enabled, it loads
// the Vite manifest from OutDir. Missing manifest is an error when Enabled.
func New(opts Options) (*Manager, error) {
	opts = normalize(opts)
	m := &Manager{opts: opts}
	if !opts.Enabled {
		return m, nil
	}
	if opts.Dev {
		return m, nil
	}
	manifest, _, err := LoadManifestFromOutDir(opts.OutDir)
	if err != nil {
		return nil, err
	}
	m.manifest = manifest
	return m, nil
}

// FromConfig builds a Manager from framework config.
func FromConfig(cfg config.IslandsConfig, isDev bool) (*Manager, error) {
	return New(OptionsFromConfig(cfg, isDev))
}

func normalize(o Options) Options {
	if o.Dir == "" {
		o.Dir = "islands"
	}
	if o.OutDir == "" {
		o.OutDir = "public/islands"
	}
	if o.DevServer == "" {
		o.DevServer = "http://localhost:5173"
	}
	if o.PublicPath == "" {
		o.PublicPath = "islands"
	}
	o.PublicPath = strings.Trim(o.PublicPath, "/")
	if o.Ext == "" {
		o.Ext = ".tsx"
	}
	if !strings.HasPrefix(o.Ext, ".") {
		o.Ext = "." + o.Ext
	}
	o.DevServer = strings.TrimRight(o.DevServer, "/")
	return o
}

// ReloadManifest re-reads the production manifest from OutDir.
func (m *Manager) ReloadManifest() error {
	if m.opts.Dev || !m.opts.Enabled {
		return nil
	}
	manifest, _, err := LoadManifestFromOutDir(m.opts.OutDir)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.manifest = manifest
	m.mu.Unlock()
	return nil
}

// Manifest returns the loaded production manifest (nil in dev or when disabled).
func (m *Manager) Manifest() Manifest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.manifest
}

// FuncMap returns helpers for html/template integration:
//
//	island(name, props?)          — mount div + scripts/CSS
//	islandRuntime()               — Vite client script in development
//	islandScripts(names...)       — scripts/CSS for named islands (no mounts)
func (m *Manager) FuncMap() template.FuncMap {
	return template.FuncMap{
		"island":        m.islandFunc,
		"islandRuntime": m.runtimeFunc,
		"islandScripts": m.scriptsFunc,
	}
}

func (m *Manager) islandFunc(name string, props ...any) (template.HTML, error) {
	var p any
	if len(props) > 0 {
		p = props[0]
	}
	return m.Render(name, p)
}

func (m *Manager) runtimeFunc() (template.HTML, error) {
	return m.Runtime(), nil
}

func (m *Manager) scriptsFunc(names ...string) (template.HTML, error) {
	return m.Scripts(names...)
}

// Render returns the mount node and associated asset tags for one island.
func (m *Manager) Render(name string, props any) (template.HTML, error) {
	if !m.opts.Enabled {
		return "", nil
	}
	if err := validateName(name); err != nil {
		return "", err
	}
	propsJSON, err := encodeProps(props)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	b.WriteString(`<div data-island="`)
	b.WriteString(html.EscapeString(name))
	b.WriteString(`" data-props='`)
	b.WriteString(propsJSON)
	b.WriteString(`'></div>`)
	assets, err := m.assetTags(name)
	if err != nil {
		return "", err
	}
	b.WriteString(string(assets))
	return template.HTML(b.String()), nil
}

// Scripts emits CSS link + module script tags for the given island names
// without mount nodes.
func (m *Manager) Scripts(names ...string) (template.HTML, error) {
	if !m.opts.Enabled || len(names) == 0 {
		return "", nil
	}
	var b bytes.Buffer
	seen := map[string]struct{}{}
	for _, name := range names {
		if err := validateName(name); err != nil {
			return "", err
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		tags, err := m.assetTags(name)
		if err != nil {
			return "", err
		}
		b.WriteString(string(tags))
	}
	return template.HTML(b.String()), nil
}

// Runtime returns the Vite HMR client script tag in development, or empty in prod.
func (m *Manager) Runtime() template.HTML {
	if !m.opts.Enabled || !m.opts.Dev {
		return ""
	}
	src := m.opts.DevServer + "/@vite/client"
	return template.HTML(`<script type="module" src="` + html.EscapeString(src) + `"></script>`)
}

func (m *Manager) assetTags(name string) (template.HTML, error) {
	if m.opts.Dev {
		src := m.devEntryURL(name)
		return template.HTML(`<script type="module" src="` + html.EscapeString(src) + `"></script>`), nil
	}

	m.mu.RLock()
	manifest := m.manifest
	m.mu.RUnlock()
	if manifest == nil {
		return "", fmt.Errorf("island: production manifest not loaded")
	}
	js, css, err := manifest.EntryAssets(name)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	for _, href := range css {
		b.WriteString(`<link rel="stylesheet" href="`)
		b.WriteString(html.EscapeString(m.publicURL(href)))
		b.WriteString(`">`)
	}
	b.WriteString(`<script type="module" src="`)
	b.WriteString(html.EscapeString(m.publicURL(js)))
	b.WriteString(`"></script>`)
	return template.HTML(b.String()), nil
}

func (m *Manager) devEntryURL(name string) string {
	// e.g. http://localhost:5173/islands/Counter.tsx
	p := path.Join("/", m.opts.Dir, name+m.opts.Ext)
	return m.opts.DevServer + p
}

func (m *Manager) publicURL(file string) string {
	file = strings.TrimLeft(strings.ReplaceAll(file, "\\", "/"), "/")
	return "/" + path.Join(m.opts.PublicPath, file)
}

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("island: empty name")
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return fmt.Errorf("island: invalid name %q", name)
	}
	return nil
}

// encodeProps JSON-encodes props for a single-quoted HTML attribute.
// Result is HTML-escaped so it is safe inside data-props='...'.
func encodeProps(props any) (string, error) {
	if props == nil {
		return "{}", nil
	}
	data, err := json.Marshal(props)
	if err != nil {
		return "", fmt.Errorf("island: encode props: %w", err)
	}
	// Escape for single-quoted attribute; also escape HTML specials.
	s := string(data)
	s = html.EscapeString(s)
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s, nil
}
