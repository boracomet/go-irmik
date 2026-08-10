// Package build pre-renders SSG / ISR / Static / CSR routes into out/.
package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boracomet/go-irmik/irmik"
	"github.com/boracomet/go-irmik/irmik/cache"
	"github.com/boracomet/go-irmik/irmik/config"
	"github.com/boracomet/go-irmik/irmik/content"
	"github.com/boracomet/go-irmik/irmik/fsutil"
	"github.com/boracomet/go-irmik/irmik/plugin"
	"github.com/boracomet/go-irmik/irmik/render"
	"github.com/boracomet/go-irmik/irmik/router"
	"github.com/boracomet/go-irmik/irmik/seo"
)

// SitemapFunc is called after HTML export to write sitemap.xml / robots.txt.
// Owned by the seo package; build invokes it when non-nil.
type SitemapFunc func(routes []router.Route, outDir string, cfg config.Config) error

// PathParamsFunc returns concrete param sets for dynamic routes at build time.
// Example: for "/blog/:slug" return []{{"slug": "hello"}, {"slug": "world"}}.
type PathParamsFunc func(rt router.Route) ([]map[string]string, error)

// Options configures a static/ISR export.
type Options struct {
	Config   config.Config
	Renderer *render.Engine
	Cache    cache.Store // optional: seed ISR entries
	Loaders  map[string]router.Loader
	// Paths resolves dynamic segments; nil means only static (no :param) routes export.
	// When nil and ContentDir is set, ContentPaths is used automatically.
	Paths PathParamsFunc
	// ContentDir enables automatic :slug expansion from Markdown collections.
	ContentDir string
	// Sitemap is optional; when nil, DefaultSitemap is used if SEO.GenerateSitemap.
	Sitemap SitemapFunc
	// Plugins run before_build / after_build when non-nil.
	Plugins *plugin.Registry
	// IslandsBuild is an optional hook to run Vite island build before HTML export.
	IslandsBuild func(cfg config.Config) error
}

// Result summarizes the export.
type Result struct {
	Wrote   []string
	Skipped []string
}

// Export discovers routes and writes pre-rendered HTML for SSG/ISR/Static/CSR modes.
func Export(ctx context.Context, opts Options) (Result, error) {
	cfg := opts.Config
	if cfg.Build.AppDir == "" {
		cfg.Build.AppDir = "app"
	}
	if cfg.Build.OutDir == "" {
		cfg.Build.OutDir = "out"
	}
	if opts.ContentDir == "" {
		opts.ContentDir = cfg.Content.Dir
	}

	pc := plugin.NewContext(ctx)
	if opts.Plugins != nil {
		if err := opts.Plugins.Run(plugin.HookBeforeBuild, pc); err != nil {
			return Result{}, fmt.Errorf("before_build: %w", err)
		}
	}

	if opts.IslandsBuild != nil {
		if err := opts.IslandsBuild(cfg); err != nil {
			return Result{}, fmt.Errorf("islands build: %w", err)
		}
	}

	if opts.Renderer == nil {
		eng, err := render.New(render.Options{
			AppDir:       cfg.Build.AppDir,
			TemplatesDir: cfg.Build.Templates,
		})
		if err != nil {
			return Result{}, err
		}
		opts.Renderer = eng
	}

	routes, err := router.Discover(cfg.Build.AppDir)
	if err != nil {
		return Result{}, err
	}

	if err := fsutil.EnsureDir(cfg.Build.OutDir); err != nil {
		return Result{}, err
	}

	// Copy public/ into out/ when present.
	if cfg.Build.PublicDir != "" {
		if err := copyDir(cfg.Build.PublicDir, filepath.Join(cfg.Build.OutDir)); err != nil && !os.IsNotExist(err) {
			return Result{}, fmt.Errorf("copy public: %w", err)
		}
	}

	pathsFn := opts.Paths
	if pathsFn == nil && opts.ContentDir != "" {
		pathsFn = ContentPaths(opts.ContentDir)
	}

	var res Result
	var sitemapEntries []seo.URLEntry
	for _, rt := range routes {
		switch rt.Meta.Mode {
		case irmik.ModeSSG, irmik.ModeISR, irmik.ModeStatic, irmik.ModeCSR:
			// export
		case irmik.ModeSSR:
			res.Skipped = append(res.Skipped, rt.URLPath+" (ssr)")
			if rt.Meta.Sitemap && !rt.Meta.NoIndex && !strings.Contains(rt.URLPath, ":") {
				sitemapEntries = append(sitemapEntries, seo.URLEntry{Loc: rt.URLPath})
			}
			continue
		default:
			res.Skipped = append(res.Skipped, rt.URLPath+" ("+string(rt.Meta.Mode)+")")
			continue
		}

		paramSets, err := expandParams(rt, pathsFn)
		if err != nil {
			return res, err
		}
		if paramSets == nil {
			res.Skipped = append(res.Skipped, rt.URLPath+" (dynamic: no Paths provider)")
			continue
		}
		for _, params := range paramSets {
			outPath, reqPath := outPaths(cfg.Build.OutDir, rt, params)
			body, err := renderOne(opts.Renderer, rt, reqPath, params, opts.Loaders)
			if err != nil {
				return res, fmt.Errorf("render %s: %w", reqPath, err)
			}
			if err := fsutil.EnsureDir(filepath.Dir(outPath)); err != nil {
				return res, err
			}
			if err := os.WriteFile(outPath, body, 0o644); err != nil {
				return res, err
			}
			res.Wrote = append(res.Wrote, outPath)

			if rt.Meta.Sitemap && !rt.Meta.NoIndex {
				sitemapEntries = append(sitemapEntries, seo.URLEntry{Loc: reqPath})
			}

			// Seed ISR cache for runtime stale-while-revalidate.
			if rt.Meta.Mode == irmik.ModeISR && opts.Cache != nil {
				revalidate := rt.Meta.Revalidate
				if revalidate <= 0 {
					revalidate = cfg.Cache.TTL
				}
				if revalidate <= 0 {
					revalidate = 60 * time.Second
				}
				now := time.Now()
				locale := cfg.I18n.DefaultLocale
				if locale == "" {
					locale = "en"
				}
				key := cache.Key("GET", reqPath, locale)
				_ = opts.Cache.Set(ctx, key, cache.Entry{
					Body:        append([]byte(nil), body...),
					ContentType: "text/html; charset=utf-8",
					StaleAt:     now.Add(revalidate),
					ExpiresAt:   now.Add(revalidate * 2),
				})
			}
		}
	}

	if opts.Sitemap != nil {
		if err := opts.Sitemap(routes, cfg.Build.OutDir, cfg); err != nil {
			return res, fmt.Errorf("sitemap: %w", err)
		}
	} else if cfg.SEO.GenerateSitemap {
		if err := DefaultSitemap(cfg.Build.OutDir, cfg, sitemapEntries); err != nil {
			return res, fmt.Errorf("sitemap: %w", err)
		}
		res.Wrote = append(res.Wrote,
			filepath.Join(cfg.Build.OutDir, "sitemap.xml"),
			filepath.Join(cfg.Build.OutDir, "robots.txt"),
		)
	}

	if opts.Plugins != nil {
		if err := opts.Plugins.Run(plugin.HookAfterBuild, pc); err != nil {
			return res, fmt.Errorf("after_build: %w", err)
		}
	}
	return res, nil
}

// ContentPaths returns a PathParamsFunc that expands :slug (and similar) from
// Markdown collections under contentDir. Heuristic: /blog/:slug → posts collection;
// otherwise uses the first path segment as the collection name.
func ContentPaths(contentDir string) PathParamsFunc {
	var store *content.Store
	var loadErr error
	ensure := func() (*content.Store, error) {
		if store != nil || loadErr != nil {
			return store, loadErr
		}
		store, loadErr = content.Load(contentDir)
		return store, loadErr
	}
	return func(rt router.Route) ([]map[string]string, error) {
		s, err := ensure()
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		param := dynamicParam(rt.URLPath)
		if param == "" {
			return []map[string]string{{}}, nil
		}
		collection := guessCollection(rt.URLPath)
		entries, err := s.List(collection)
		if err != nil {
			// Try "posts" as a blog fallback.
			if collection != "posts" {
				entries, err = s.List("posts")
			}
			if err != nil {
				return nil, nil // skip rather than fail the whole build
			}
		}
		out := make([]map[string]string, 0, len(entries))
		for _, e := range entries {
			out = append(out, map[string]string{param: e.Slug})
		}
		return out, nil
	}
}

func guessCollection(urlPath string) string {
	parts := strings.Split(strings.Trim(urlPath, "/"), "/")
	if len(parts) == 0 {
		return "posts"
	}
	seg := parts[0]
	switch seg {
	case "blog", "posts", "articles":
		return "posts"
	default:
		return seg
	}
}

func dynamicParam(urlPath string) string {
	for _, p := range strings.Split(urlPath, "/") {
		if strings.HasPrefix(p, ":") {
			return strings.TrimPrefix(p, ":")
		}
		if strings.HasPrefix(p, "*") {
			return strings.TrimPrefix(p, "*")
		}
	}
	return ""
}

// DefaultSitemap writes sitemap.xml and robots.txt using the seo package.
func DefaultSitemap(outDir string, cfg config.Config, entries []seo.URLEntry) error {
	xml, err := seo.SitemapXML(cfg.App.BaseURL, entries)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "sitemap.xml"), xml, 0o644); err != nil {
		return err
	}
	robots := seo.RobotsTxt(cfg.App.BaseURL, cfg.SEO.GenerateSitemap)
	return os.WriteFile(filepath.Join(outDir, "robots.txt"), []byte(robots), 0o644)
}

func expandParams(rt router.Route, paths PathParamsFunc) ([]map[string]string, error) {
	dynamic := strings.Contains(rt.URLPath, ":") || strings.Contains(rt.URLPath, "*")
	if !dynamic {
		return []map[string]string{{}}, nil
	}
	if paths == nil {
		return nil, nil // skip dynamic without param provider
	}
	return paths(rt)
}

func outPaths(outDir string, rt router.Route, params map[string]string) (filePath, reqPath string) {
	reqPath = materializePath(rt.URLPath, params)
	if reqPath == "/" {
		return filepath.Join(outDir, "index.html"), "/"
	}
	// /about → out/about/index.html — path segments are already URL-safe;
	// use SafeFileName only for the final segment when it could contain odd chars.
	rel := strings.TrimPrefix(reqPath, "/")
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		parts[i] = fsutil.SafeFileName(p)
	}
	safeRel := strings.Join(parts, string(filepath.Separator))
	return filepath.Join(outDir, safeRel, "index.html"), reqPath
}

func materializePath(pattern string, params map[string]string) string {
	if pattern == "/" {
		return "/"
	}
	parts := strings.Split(strings.Trim(pattern, "/"), "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.HasPrefix(p, ":") {
			key := strings.TrimPrefix(p, ":")
			out = append(out, params[key])
			continue
		}
		if strings.HasPrefix(p, "*") {
			key := strings.TrimPrefix(p, "*")
			out = append(out, params[key])
			continue
		}
		out = append(out, p)
	}
	return "/" + strings.Join(out, "/")
}

func renderOne(eng *render.Engine, rt router.Route, reqPath string, params map[string]string, loaders map[string]router.Loader) ([]byte, error) {
	var data any
	if loaders != nil {
		if loader, ok := loaders[rt.URLPath]; ok && loader != nil {
			_ = loader
			data = map[string]any{"params": params, "path": reqPath}
		}
	}
	if data == nil {
		data = map[string]any{"params": params, "path": reqPath}
	}
	return eng.RenderToBytes(rt.PageFile, rt.LayoutFiles, render.Data{
		Meta:   rt.Meta,
		Data:   data,
		Params: params,
		Path:   reqPath,
	})
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return err
			}
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}
