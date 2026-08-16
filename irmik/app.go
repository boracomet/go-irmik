package irmik

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/gin-gonic/gin"

	"github.com/boracomet/go-irmik/irmik/auth"
	"github.com/boracomet/go-irmik/irmik/cache"
	"github.com/boracomet/go-irmik/irmik/config"
	"github.com/boracomet/go-irmik/irmik/health"
	"github.com/boracomet/go-irmik/irmik/island"
	"github.com/boracomet/go-irmik/irmik/lifecycle"
	"github.com/boracomet/go-irmik/irmik/middleware"
	"github.com/boracomet/go-irmik/irmik/plugin"
	"github.com/boracomet/go-irmik/irmik/render"
	"github.com/boracomet/go-irmik/irmik/router"
	"github.com/boracomet/go-irmik/irmik/session"
	"github.com/boracomet/go-irmik/irmik/tmplfunc"
)

// App is the Irmik application root: Gin engine, cache, and plugin registry.
type App struct {
	Config   config.Config
	Engine   *gin.Engine
	Cache    cache.Store
	Plugins  *plugin.Registry
	Router   *router.Router
	Renderer *render.Engine
	Islands  *island.Manager
	// Sessions is optional cookie session manager (EnableSessions).
	Sessions *session.Manager
	// Auth is optional authenticator (EnableAuth); JWT + session helpers.
	Auth *auth.Authenticator

	ready       atomic.Bool
	readyChecks *health.Registry
	srv         *http.Server
}

// MountOptions configures file-based page mounting.
type MountOptions struct {
	Loaders map[string]router.Loader
	// Funcs are extra template helpers (SEO, etc.).
	Funcs template.FuncMap
	// SkipIslands disables automatic island.FromConfig wiring.
	SkipIslands bool
}

// New constructs an App from cfg: Gin engine, default middleware, health
// routes, cache store, and an empty plugin registry.
func New(cfg config.Config) (*App, error) {
	if !cfg.IsDev() && weakSecret(cfg.Auth.JWTSecret) {
		return nil, errors.New("irmik: auth.jwtSecret must be set to a non-demo value outside development")
	}
	if cfg.IsDev() {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	store, err := cache.New(cache.Options{
		Driver:   cfg.Cache.Driver,
		TTL:      cfg.Cache.TTL,
		DiskDir:  cfg.Cache.DiskDir,
		RedisURL: cfg.Cache.RedisURL,
	})
	if err != nil {
		return nil, fmt.Errorf("irmik: cache: %w", err)
	}

	engine := gin.New()
	engine.Use(middleware.Recovery(), middleware.RequestID())

	// Cheap baseline headers are on by default; rate limit stays opt-in.
	sec := middleware.DefaultSecureHeaders()
	if !cfg.IsDev() {
		sec.HSTSMaxAge = 31536000
		sec.HSTSIncludeSubdomains = true
		sec.FrameAncestors = "'none'"
	}
	engine.Use(middleware.SecureHeaders(sec))

	if len(cfg.Server.TrustedProxies) > 0 {
		if err := engine.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
			return nil, fmt.Errorf("irmik: trustedProxies: %w", err)
		}
	} else if !cfg.IsDev() {
		if err := engine.SetTrustedProxies(nil); err != nil {
			return nil, fmt.Errorf("irmik: disable trusted proxies: %w", err)
		}
	}

	app := &App{
		Config:      cfg,
		Engine:      engine,
		Cache:       store,
		Plugins:     plugin.NewRegistry(),
		readyChecks: health.New(),
	}

	middleware.HealthWith(engine, middleware.HealthConfig{
		Ready:  app.lifecycleReady,
		Checks: app.readyChecks,
	})
	return app, nil
}

func weakSecret(secret string) bool {
	switch strings.TrimSpace(secret) {
	case "", "dev-only-change-me-jwt-secret-32b", "change-me", "secret", "password123":
		return true
	default:
		return false
	}
}

// EnableSecureDefaults turns on admin-oriented protections beyond baseline headers:
// global in-memory rate limiting. Pair with csrf.Middleware for browser form/admin UIs.
// Headers are already applied in New; production also gets HSTS.
func (a *App) EnableSecureDefaults() {
	a.EnableRateLimit(middleware.DefaultRateLimit())
}

// EnableRateLimit mounts an in-memory token-bucket limiter (per ClientIP by default).
// For login/auth routes, prefer middleware.LoginRateLimit on those routes instead of
// (or in addition to) a loose global limit.
func (a *App) EnableRateLimit(cfg middleware.RateLimitConfig) {
	a.Engine.Use(middleware.RateLimit(cfg))
}

// EnableSecureHeaders remounts security headers with a custom config
// (e.g. CSP frame-ancestors). Prefer calling early after New.
func (a *App) EnableSecureHeaders(cfg middleware.SecureHeadersConfig) {
	a.Engine.Use(middleware.SecureHeaders(cfg))
}

// UseRequestLog mounts structured slog request logging (method, path, status,
// latency, request-id). Opt-in; uses slog.Default() when logger is nil.
func (a *App) UseRequestLog() {
	a.UseRequestLogWith(nil)
}

// UseRequestLogWith is UseRequestLog with an explicit logger.
func (a *App) UseRequestLogWith(logger *slog.Logger) {
	a.Engine.Use(middleware.RequestLog(logger))
}

// RegisterReadyCheck adds a required dependency probe for /ready.
// /health remains liveness-only. Example:
//
//	app.RegisterReadyCheck("db", health.PingDB(db))
func (a *App) RegisterReadyCheck(name string, fn health.CheckFunc) {
	if a.readyChecks == nil {
		a.readyChecks = health.New()
	}
	a.readyChecks.Register(name, fn)
}

// RegisterOptionalReadyCheck adds a probe reported on /ready but ignored for readiness.
func (a *App) RegisterOptionalReadyCheck(name string, fn health.CheckFunc) {
	if a.readyChecks == nil {
		a.readyChecks = health.New()
	}
	a.readyChecks.RegisterOptional(name, fn)
}

// ReadyChecks returns the readiness registry (may be empty, never nil after New).
func (a *App) ReadyChecks() *health.Registry {
	if a.readyChecks == nil {
		a.readyChecks = health.New()
	}
	return a.readyChecks
}

// EnableSessions constructs a session.Manager from cfg.Session and mounts
// its middleware on the Gin engine. Safe to call once after New.
func (a *App) EnableSessions() error {
	if a.Sessions != nil {
		return nil
	}
	sc := a.Config.Session
	mgr, err := session.NewManager(session.Options{
		Name:     sc.Name,
		Secret:   sc.Secret,
		MaxAge:   sc.MaxAge,
		Path:     sc.Path,
		Domain:   sc.Domain,
		Secure:   a.Config.SessionSecure(),
		HTTPOnly: a.Config.SessionHTTPOnly(),
		SameSite: sc.SameSite,
		Driver:   sc.Driver,
		RedisURL: sc.RedisURL,
	})
	if err != nil {
		return fmt.Errorf("irmik: sessions: %w", err)
	}
	a.Sessions = mgr
	a.Engine.Use(mgr.Middleware())
	return nil
}

// EnableAuth constructs an auth.Authenticator from cfg.Auth.
// Does not mount middleware; call Auth.InjectSessionUser / MiddlewareJWT as needed.
func (a *App) EnableAuth() *auth.Authenticator {
	if a.Auth != nil {
		return a.Auth
	}
	ac := a.Config.Auth
	a.Auth = auth.New(auth.Config{
		JWTSecret: ac.JWTSecret,
		JWTIssuer: ac.JWTIssuer,
		AccessTTL: ac.AccessTTL,
	})
	return a.Auth
}

// MountPages creates the renderer (if needed), discovers app/ routes, and binds Gin handlers.
func (a *App) MountPages(opts MountOptions) error {
	cfg := a.Config

	funcs := tmplfunc.Map()
	for k, v := range opts.Funcs {
		funcs[k] = v
	}

	var islandMgr *island.Manager
	if !opts.SkipIslands && cfg.Islands.Enabled {
		mgr, err := island.FromConfig(cfg.Islands, cfg.IsDev())
		if err != nil {
			// Soft-fail when the Vite manifest is missing (before first islands build).
			mgr, err = island.New(island.Options{
				Enabled:   true,
				Dev:       true,
				DevServer: cfg.Islands.DevServer,
				Dir:       cfg.Islands.Dir,
				OutDir:    cfg.Islands.OutDir,
			})
		}
		if err == nil && mgr != nil {
			islandMgr = mgr
			a.Islands = mgr
			for k, v := range mgr.FuncMap() {
				if k == "island" {
					continue // wired via SetIslandFunc below
				}
				funcs[k] = v
			}
		}
	}

	if a.Renderer == nil {
		eng, err := render.New(render.Options{
			AppDir:       cfg.Build.AppDir,
			TemplatesDir: cfg.Build.Templates,
			Funcs:        funcs,
		})
		if err != nil {
			return fmt.Errorf("irmik: renderer: %w", err)
		}
		a.Renderer = eng
	} else {
		a.Renderer.SetFuncs(funcs)
	}

	if islandMgr != nil {
		mgr := islandMgr
		a.Renderer.SetIslandFunc(func(name string, props any) (template.HTML, error) {
			return mgr.Render(name, props)
		})
	}

	// Serve built island assets at /islands/* (public/ is mounted by the router).
	if cfg.Islands.OutDir != "" {
		if st, err := os.Stat(cfg.Islands.OutDir); err == nil && st.IsDir() {
			a.Engine.Static("/islands", cfg.Islands.OutDir)
		}
	}

	locale := cfg.I18n.DefaultLocale
	if locale == "" {
		locale = "en"
	}
	rt, err := router.New(router.Options{
		AppDir:    cfg.Build.AppDir,
		OutDir:    cfg.Build.OutDir,
		PublicDir: cfg.Build.PublicDir,
		Locale:    locale,
		Cache:     a.Cache,
		Renderer:  a.Renderer,
		Loaders:   opts.Loaders,
	})
	if err != nil {
		return fmt.Errorf("irmik: router: %w", err)
	}
	a.Router = rt
	rt.Mount(a.Engine)
	return nil
}

// RemountPages reloads templates and rediscovers routes in memory.
// Gin handlers are not re-bound (duplicate registration); new routes need a process restart.
func (a *App) RemountPages() error {
	if a.Renderer != nil {
		if err := a.Renderer.Reload(); err != nil {
			return err
		}
	}
	if a.Islands != nil {
		_ = a.Islands.ReloadManifest()
	}
	if a.Router == nil {
		return a.MountPages(MountOptions{})
	}
	routes, err := router.Discover(a.Config.Build.AppDir)
	if err != nil {
		return err
	}
	a.Router.SetRoutes(routes)
	return nil
}

// lifecycleReady is the process gate used by /ready (startup finished).
func (a *App) lifecycleReady() bool {
	return a.ready.Load()
}

// Ready reports whether the app has finished starting and required dependency
// checks pass. Used by callers; /ready also runs Checks via HealthWith.
func (a *App) Ready() bool {
	if !a.ready.Load() {
		return false
	}
	if a.readyChecks == nil {
		return true
	}
	return a.readyChecks.Ready(context.Background())
}

// Use registers a plugin on the app registry.
func (a *App) Use(p plugin.Plugin) error {
	return a.Plugins.Use(p)
}

// Run listens on cfg.Server address until ctx is cancelled (or SIGINT/SIGTERM),
// then shuts down gracefully using Server.ShutdownTimeout.
// Plugin hooks: before_start → after_start → (serve) → before_stop → after_stop.
func (a *App) Run(ctx context.Context) error {
	pc := plugin.NewContext(ctx)
	if err := a.Plugins.Run(plugin.HookBeforeStart, pc); err != nil {
		return fmt.Errorf("irmik: before_start: %w", err)
	}

	a.srv = &http.Server{
		Addr:         a.Config.Addr(),
		Handler:      a.Engine,
		ReadTimeout:  a.Config.Server.ReadTimeout,
		WriteTimeout: a.Config.Server.WriteTimeout,
	}

	runCtx, stop := lifecycle.Signals(ctx)
	defer stop()

	a.ready.Store(true)
	_ = a.Plugins.Run(plugin.HookAfterStart, plugin.NewContext(runCtx))

	err := lifecycle.Serve(runCtx, a.srv, a.Config.Server.ShutdownTimeout)

	a.ready.Store(false)
	stopPC := plugin.NewContext(context.Background())
	_ = a.Plugins.Run(plugin.HookBeforeStop, stopPC)

	if a.Cache != nil {
		_ = a.Cache.Close()
	}
	if a.Sessions != nil {
		_ = a.Sessions.Close()
	}
	_ = a.Plugins.Run(plugin.HookAfterStop, stopPC)

	if err != nil {
		return fmt.Errorf("irmik: serve: %w", err)
	}
	return nil
}

// AdaptLoader converts an Irmik Context loader into a router.Loader.
func AdaptLoader(fn func(*Context) (any, error)) router.Loader {
	return func(c *gin.Context) (any, error) {
		return fn(FromGin(c))
	}
}

// HTTPServer returns the underlying http.Server after Run has started it.
func (a *App) HTTPServer() *http.Server {
	return a.srv
}
