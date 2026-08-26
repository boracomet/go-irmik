// Package cli implements the irmik command-line interface.
package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/boracomet/go-irmik/internal/build"
	"github.com/boracomet/go-irmik/internal/hmr"
	"github.com/boracomet/go-irmik/irmik"
	"github.com/boracomet/go-irmik/irmik/cache"
	"github.com/boracomet/go-irmik/irmik/config"
	"github.com/boracomet/go-irmik/irmik/router"

	// CLI links optional drivers so cache clear / migrate work without app blank-imports.
	_ "github.com/boracomet/go-irmik/irmik/cache/redisx"
	_ "github.com/boracomet/go-irmik/irmik/db/mysql"
	_ "github.com/boracomet/go-irmik/irmik/db/postgres"
	_ "github.com/boracomet/go-irmik/irmik/db/sqlite"
	_ "github.com/boracomet/go-irmik/irmik/session/redisx"
)

// ScaffoldModuleVersion is the go-irmik version pinned by `irmik new`.
const ScaffoldModuleVersion = "v0.1.1"

// Execute runs the root command.
func Execute() error {
	return Root().Execute()
}

// Root builds the cobra command tree.
func Root() *cobra.Command {
	root := &cobra.Command{
		Use:           "irmik",
		Short:         "Irmik — Gin meta-framework CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringP("config", "c", "irmik.yaml", "path to irmik.yaml")
	root.AddCommand(cmdNew(), cmdGenerate(), cmdDev(), cmdBuild(), cmdStart(), cmdCache(), cmdMigrate())
	return root
}

func cmdNew() *cobra.Command {
	var admin bool
	c := &cobra.Command{
		Use:   "new <name>",
		Short: "Create a small Irmik project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := filepath.Base(strings.TrimSpace(args[0]))
			if name == "" || name == "." || name == ".." {
				return fmt.Errorf("invalid project name")
			}
			dir := name
			if err := os.Mkdir(dir, 0o755); err != nil {
				return err
			}
			files := map[string]string{
				"go.mod":       scaffoldGoMod(name),
				"irmik.yaml":   "app:\n  name: " + name + "\n  env: development\nserver:\n  host: 127.0.0.1\n  port: 8080\n",
				".env.example": "# Production only: openssl rand -base64 32\nIRMIK_JWT_SECRET=\n",
				".gitignore":   ".env\nout/\ndata/\n",
				"README.md":    "# " + name + "\n\n```sh\ngo run .\n# open http://127.0.0.1:8080\n```\n",
				"main.go":      scaffoldMain(),
			}
			if admin {
				files["README.md"] += "\nThe `--admin` starter adds a minimal in-memory admin surface; configure a real identity store before production.\n"
			}
			for path, body := range files {
				if err := os.WriteFile(filepath.Join(dir, path), []byte(body), 0o644); err != nil {
					return err
				}
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", dir)
			return nil
		},
	}
	c.Flags().BoolVar(&admin, "admin", false, "include the admin starter")
	return c
}

func scaffoldGoMod(moduleName string) string {
	return "module " + moduleName + "\n\ngo 1.25.0\n\nrequire github.com/boracomet/go-irmik " + ScaffoldModuleVersion + "\n"
}

func scaffoldMain() string {
	return `package main

import (
  "context"
  "github.com/gin-gonic/gin"
  "github.com/boracomet/go-irmik/irmik"
  "github.com/boracomet/go-irmik/irmik/config"
)

func main() {
  cfg := config.Default()
  app, err := irmik.New(cfg); if err != nil { panic(err) }
  app.EnableSecureDefaults()
  app.Engine.GET("/", func(c *gin.Context) { c.String(200, "Hello from Irmik") })
  if err := app.Run(context.Background()); err != nil { panic(err) }
}
`
}

func loadCfg(cmd *cobra.Command) (config.Config, error) {
	path, _ := cmd.Flags().GetString("config")
	return config.Load(path)
}

func cmdGenerate() *cobra.Command {
	var (
		appDir  string
		outFile string
		pkg     string
	)
	c := &cobra.Command{
		Use:   "generate",
		Short: "Scan app/ and write zz_routes_gen.go",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadCfg(cmd)
			if err != nil {
				return err
			}
			if appDir == "" {
				appDir = cfg.Build.AppDir
			}
			if outFile == "" {
				outFile = "zz_routes_gen.go"
			}
			if err := router.Generate(router.GenerateOptions{
				AppDir:      appDir,
				OutFile:     outFile,
				PackageName: pkg,
			}); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", outFile)
			return nil
		},
	}
	c.Flags().StringVar(&appDir, "app", "", "app directory (default from config)")
	c.Flags().StringVar(&outFile, "out", "", "output Go file (default zz_routes_gen.go)")
	c.Flags().StringVar(&pkg, "package", "main", "Go package name for generated file")
	return c
}

func cmdCache() *cobra.Command {
	c := &cobra.Command{
		Use:   "cache",
		Short: "Cache utilities",
	}
	clear := &cobra.Command{
		Use:   "clear",
		Short: "Clear the configured cache store (memory/disk/redis)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadCfg(cmd)
			if err != nil {
				return err
			}
			store, err := cache.New(cache.Options{
				Driver:   cfg.Cache.Driver,
				TTL:      cfg.Cache.TTL,
				DiskDir:  cfg.Cache.DiskDir,
				RedisURL: cfg.Cache.RedisURL,
			})
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			if err := store.Clear(ctx); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "cache cleared (driver=%s)\n", cfg.CacheDriver())
			return nil
		},
	}
	c.AddCommand(clear)
	return c
}

func cmdBuild() *cobra.Command {
	c := &cobra.Command{
		Use:   "build",
		Short: "Pre-render SSG/ISR/Static/CSR routes into out/",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadCfg(cmd)
			if err != nil {
				return err
			}
			cfg.App.Env = "production"

			// Build islands first so MountPages can load the Vite manifest.
			if cfg.Islands.Enabled {
				if err := runIslandsBuild(cmd.OutOrStdout(), cfg); err != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "islands build warning: %v\n", err)
				}
			}

			app, err := irmik.New(cfg)
			if err != nil {
				return err
			}
			if err := app.MountPages(irmik.MountOptions{}); err != nil {
				return err
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			res, err := build.Export(ctx, build.Options{
				Config:     cfg,
				Renderer:   app.Renderer,
				Cache:      app.Cache,
				Plugins:    app.Plugins,
				ContentDir: cfg.Content.Dir,
				// Islands already built above; skip second pass.
				IslandsBuild: nil,
			})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "build: wrote %d file(s)\n", len(res.Wrote))
			for _, f := range res.Wrote {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", f)
			}
			for _, s := range res.Skipped {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  skip %s\n", s)
			}
			return nil
		},
	}
	return c
}

func runIslandsBuild(w io.Writer, cfg config.Config) error {
	if _, err := os.Stat("package.json"); err != nil {
		_, _ = fmt.Fprintln(w, "islands: no package.json — skip Vite build")
		return nil
	}
	_, _ = fmt.Fprintln(w, "islands: running npm run build…")
	cmd := exec.Command("npm", "run", "build")
	cmd.Stdout = w
	cmd.Stderr = w
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm run build: %w", err)
	}
	_, _ = fmt.Fprintf(w, "islands: built → %s\n", cfg.Islands.OutDir)
	return nil
}

func cmdStart() *cobra.Command {
	c := &cobra.Command{
		Use:   "start",
		Short: "Production serve (out/ + SSR/ISR runtime)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadCfg(cmd)
			if err != nil {
				return err
			}
			if cfg.App.Env == "" || cfg.IsDev() {
				cfg.App.Env = "production"
			}

			app, err := irmik.New(cfg)
			if err != nil {
				return err
			}
			if err := app.MountPages(irmik.MountOptions{}); err != nil {
				return err
			}

			// Serve exported static files at / for assets; page routes take precedence.
			if cfg.Build.OutDir != "" {
				if st, err := os.Stat(cfg.Build.OutDir); err == nil && st.IsDir() {
					app.Engine.Static("/assets", filepath.Join(cfg.Build.OutDir, "assets"))
					app.Engine.StaticFile("/sitemap.xml", filepath.Join(cfg.Build.OutDir, "sitemap.xml"))
					app.Engine.StaticFile("/robots.txt", filepath.Join(cfg.Build.OutDir, "robots.txt"))
				}
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "irmik start listening on %s\n", cfg.Addr())
			return app.Run(ctx)
		},
	}
	return c
}

func cmdDev() *cobra.Command {
	c := &cobra.Command{
		Use:   "dev",
		Short: "Development server with fsnotify HMR (Vite spawn stub)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadCfg(cmd)
			if err != nil {
				return err
			}
			cfg.App.Env = "development"

			app, err := irmik.New(cfg)
			if err != nil {
				return err
			}
			if err := app.MountPages(irmik.MountOptions{}); err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			var viteCmd *exec.Cmd
			if cfg.Islands.Enabled {
				viteCmd = startViteStub(cmd.OutOrStdout(), cfg)
				if viteCmd != nil {
					defer func() {
						if viteCmd.Process != nil {
							_ = viteCmd.Process.Signal(os.Interrupt)
						}
					}()
				}
			}

			go func() {
				dirs := []string{cfg.Build.AppDir, cfg.Build.Templates}
				_ = hmr.Watch(ctx, hmr.Options{
					Dirs:       dirs,
					Extensions: []string{".html", ".yaml", ".yml"},
					Debounce:   200 * time.Millisecond,
				}, func(ev hmr.Event) {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "hmr: %s (%s)\n", ev.Path, ev.Op)
					var reloadErr error
					if app.Renderer != nil {
						reloadErr = app.Renderer.Reload()
						if reloadErr != nil {
							_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "hmr reload: %v\n", reloadErr)
						}
					}
					if err := app.RemountPages(); err != nil {
						_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "hmr remount: %v\n", err)
						if reloadErr == nil {
							reloadErr = err
						}
					}
					if app.Devtools == nil {
						return
					}
					if reloadErr != nil {
						app.Devtools.Report("template", reloadErr.Error())
						return
					}
					app.Devtools.Reload(ev.Path)
				})
			}()

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "irmik dev listening on %s\n", cfg.Addr())
			if cfg.Islands.Enabled {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "islands dev server expected at %s\n", cfg.Islands.DevServer)
			}
			return app.Run(ctx)
		},
	}
	return c
}

// startViteStub tries to spawn `npx vite` when package.json exists; otherwise logs a hook message.
func startViteStub(w io.Writer, cfg config.Config) *exec.Cmd {
	if _, err := os.Stat("package.json"); err != nil {
		_, _ = fmt.Fprintln(w, "vite: no package.json — skip spawn (island package can hook here)")
		return nil
	}
	cmd := exec.Command("npx", "--yes", "vite", "--port", "5173")
	cmd.Stdout = w
	cmd.Stderr = w
	if err := cmd.Start(); err != nil {
		_, _ = fmt.Fprintf(w, "vite: spawn failed: %v (continuing without Vite)\n", err)
		return nil
	}
	_, _ = fmt.Fprintf(w, "vite: started pid=%d (devServer=%s)\n", cmd.Process.Pid, cfg.Islands.DevServer)
	go func() {
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			resp, err := http.Get(cfg.Islands.DevServer)
			if err == nil {
				_ = resp.Body.Close()
				return
			}
			time.Sleep(300 * time.Millisecond)
		}
	}()
	return cmd
}
