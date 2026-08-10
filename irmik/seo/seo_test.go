package seo_test

import (
	"strings"
	"testing"
	"time"

	"github.com/boracomet/go-irmik/irmik/config"
	"github.com/boracomet/go-irmik/irmik/seo"
)

func TestTitleCanonicalOGTwitter(t *testing.T) {
	cfg := config.Default()
	cfg.App.BaseURL = "https://example.com"
	cfg.SEO.SiteName = "Irmik"
	cfg.SEO.DefaultOGImage = "/og.png"
	cfg.SEO.TwitterHandle = "irmikdev"

	site := seo.NewSite(cfg)
	if got := site.Title("About"); got != "About | Irmik" {
		t.Fatalf("Title = %q", got)
	}
	if got := site.Canonical("/about"); got != "https://example.com/about" {
		t.Fatalf("Canonical = %q", got)
	}

	m := seo.Meta{
		Title:       "About",
		Description: "About us",
		Canonical:   "/about",
	}
	og := site.OpenGraph(m)
	if og["og:title"] != "About | Irmik" {
		t.Errorf("og:title = %q", og["og:title"])
	}
	if og["og:url"] != "https://example.com/about" {
		t.Errorf("og:url = %q", og["og:url"])
	}
	if og["og:image"] != "https://example.com/og.png" {
		t.Errorf("og:image = %q", og["og:image"])
	}
	tw := site.Twitter(m)
	if tw["twitter:site"] != "@irmikdev" {
		t.Errorf("twitter:site = %q", tw["twitter:site"])
	}
	if tw["twitter:card"] != "summary_large_image" {
		t.Errorf("twitter:card = %q", tw["twitter:card"])
	}
}

func TestJSONLDAndHeadHTML(t *testing.T) {
	site := seo.Site{BaseURL: "https://example.com", SiteName: "Irmik"}
	raw, err := seo.JSONLD(map[string]any{"@type": "WebSite", "name": "Irmik"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, `"name":"Irmik"`) {
		t.Fatalf("jsonld = %s", raw)
	}
	html, err := site.HeadHTML(seo.Meta{Title: "Home", Description: "Hi", Canonical: "/"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<title>Home | Irmik</title>",
		`name="description"`,
		`rel="canonical"`,
		`property="og:title"`,
		`name="twitter:card"`,
		`application/ld+json`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("HeadHTML missing %q in:\n%s", want, html)
		}
	}
}

func TestSitemapXML(t *testing.T) {
	mod := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	xml, err := seo.SitemapXML("https://example.com", []seo.URLEntry{
		(seo.URLEntry{Loc: "/about", LastMod: &mod, ChangeFreq: "monthly"}).WithPriority(0.8),
		{Loc: "/"},
		{Loc: "https://example.com/blog/hello"},
		{Loc: "/about"}, // duplicate
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(xml)
	if !strings.HasPrefix(s, xmlHeader()) {
		t.Fatalf("missing xml header: %q", s[:min(40, len(s))])
	}
	if !strings.Contains(s, `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`) {
		t.Fatalf("missing urlset: %s", s)
	}
	if strings.Count(s, "<loc>https://example.com/about</loc>") != 1 {
		t.Fatalf("expected one about loc: %s", s)
	}
	if !strings.Contains(s, "<lastmod>2026-08-10</lastmod>") {
		t.Fatalf("missing lastmod: %s", s)
	}
	if !strings.Contains(s, "<priority>0.8</priority>") {
		t.Fatalf("missing priority: %s", s)
	}
	if !strings.Contains(s, "<loc>https://example.com/</loc>") {
		t.Fatalf("missing home: %s", s)
	}
	if !strings.Contains(s, "<loc>https://example.com/blog/hello</loc>") {
		t.Fatalf("missing blog: %s", s)
	}
}

func TestSitemapFromPathsAndRobots(t *testing.T) {
	xml, err := seo.SitemapFromPaths("https://example.com", []string{"/", "/blog", " /blog "})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(xml), "<loc>") != 2 {
		t.Fatalf("expected 2 urls after dedupe: %s", xml)
	}

	robots := seo.RobotsTxt("https://example.com", true)
	if !strings.Contains(robots, "User-agent: *") || !strings.Contains(robots, "Allow: /") {
		t.Fatalf("robots = %q", robots)
	}
	if !strings.Contains(robots, "Sitemap: https://example.com/sitemap.xml") {
		t.Fatalf("robots missing sitemap: %q", robots)
	}
	noSM := seo.RobotsTxt("https://example.com", false)
	if strings.Contains(noSM, "Sitemap:") {
		t.Fatalf("expected no sitemap line: %q", noSM)
	}
}

func xmlHeader() string { return `<?xml version="1.0" encoding="UTF-8"?>` }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
