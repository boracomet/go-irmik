package main

import (
	"context"
	"fmt"
	"html/template"
	"os"
	"os/signal"
	"syscall"

	"github.com/boracomet/go-irmik/irmik"
	"github.com/boracomet/go-irmik/irmik/config"
	"github.com/boracomet/go-irmik/irmik/content"
	"github.com/boracomet/go-irmik/irmik/render"
	"github.com/boracomet/go-irmik/irmik/router"
	"github.com/boracomet/go-irmik/irmik/seo"
)

type postMeta struct {
	Title       string `yaml:"title" toml:"title" json:"title"`
	Description string `yaml:"description" toml:"description" json:"description"`
	Date        string `yaml:"date" toml:"date" json:"date"`
	Draft       bool   `yaml:"draft" toml:"draft" json:"draft"`
}

func main() {
	cfg, err := config.Load("irmik.yaml")
	if err != nil {
		fatal(err)
	}

	store, err := content.Load(cfg.Content.Dir)
	if err != nil {
		fatal(err)
	}

	site := seo.NewSite(cfg)

	app, err := irmik.New(cfg)
	if err != nil {
		fatal(err)
	}

	if err := app.MountPages(irmik.MountOptions{
		Funcs: template.FuncMap{
			"seoHead": func(d render.Data) (template.HTML, error) {
				title, desc := "Irmik Blog", ""
				if m, ok := d.Data.(map[string]any); ok {
					if t, ok := m["Title"].(string); ok && t != "" {
						title = t
					}
					if s, ok := m["Description"].(string); ok {
						desc = s
					}
				}
				canon := d.Path
				html, err := site.HeadHTML(seo.Meta{
					Title:       title,
					Description: desc,
					Canonical:   canon,
				})
				return template.HTML(html), err
			},
		},
		Loaders: map[string]router.Loader{
			"/": irmik.AdaptLoader(func(c *irmik.Context) (any, error) {
				return map[string]any{
					"Title":       "Home",
					"Description": "A sample blog built with Irmik",
				}, nil
			}),
			"/about": irmik.AdaptLoader(func(c *irmik.Context) (any, error) {
				return map[string]any{
					"Title": "About",
				}, nil
			}),
			"/blog": irmik.AdaptLoader(func(c *irmik.Context) (any, error) {
				docs, err := content.List[postMeta](store, "posts")
				if err != nil {
					return nil, err
				}
				type item struct {
					Slug        string
					Title       string
					Description string
					Date        string
				}
				posts := make([]item, 0, len(docs))
				for _, d := range docs {
					if d.Meta.Draft {
						continue
					}
					posts = append(posts, item{
						Slug:        d.Slug,
						Title:       d.Meta.Title,
						Description: d.Meta.Description,
						Date:        d.Meta.Date,
					})
				}
				return map[string]any{
					"Title": "Blog",
					"Posts": posts,
				}, nil
			}),
			"/blog/:slug": irmik.AdaptLoader(func(c *irmik.Context) (any, error) {
				slug := c.Param("slug")
				doc, err := content.Get[postMeta](store, "posts", slug)
				if err != nil {
					return nil, err
				}
				return map[string]any{
					"Title":       doc.Meta.Title,
					"Description": doc.Meta.Description,
					"Date":        doc.Meta.Date,
					"Body":        template.HTML(doc.Body),
					"Slug":        doc.Slug,
				}, nil
			}),
		},
	}); err != nil {
		fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("irmik blog listening on http://%s\n", cfg.Addr())
	if err := app.Run(ctx); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "blog: %v\n", err)
	os.Exit(1)
}
