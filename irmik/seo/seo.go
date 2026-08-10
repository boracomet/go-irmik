// Package seo builds page meta tags, JSON-LD, sitemaps, and robots.txt.
package seo

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/boracomet/go-irmik/irmik/config"
)

// Meta holds page-level SEO fields. Empty strings fall back to site defaults where applicable.
type Meta struct {
	Title       string
	Description string
	Canonical   string // absolute URL or site-relative path
	Image       string // absolute URL or site-relative path
	Type        string // og:type, default "website"
	Robots      string
	TwitterCard string // default "summary_large_image"
	JSONLD      any    // optional structured data object (marshaled as script)
}

// Site carries site-wide SEO defaults from config.
type Site struct {
	BaseURL       string
	SiteName      string
	DefaultImage  string
	TwitterHandle string
}

// NewSite builds Site from framework config.
func NewSite(cfg config.Config) Site {
	return Site{
		BaseURL:       strings.TrimRight(cfg.App.BaseURL, "/"),
		SiteName:      cfg.SEO.SiteName,
		DefaultImage:  cfg.SEO.DefaultOGImage,
		TwitterHandle: cfg.SEO.TwitterHandle,
	}
}

// Title returns "Page | SiteName" when both are set, otherwise the non-empty part.
func (s Site) Title(pageTitle string) string {
	pageTitle = strings.TrimSpace(pageTitle)
	site := strings.TrimSpace(s.SiteName)
	switch {
	case pageTitle == "":
		return site
	case site == "" || strings.EqualFold(pageTitle, site):
		return pageTitle
	default:
		return pageTitle + " | " + site
	}
}

// Description returns a trimmed description (pass-through helper for templates).
func (s Site) Description(desc string) string {
	return strings.TrimSpace(desc)
}

// AbsoluteURL joins a path or returns an already-absolute URL.
func (s Site) AbsoluteURL(href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return s.BaseURL
	}
	if isAbsoluteURL(href) {
		return href
	}
	if !strings.HasPrefix(href, "/") {
		href = "/" + href
	}
	return s.BaseURL + href
}

// Canonical resolves a page path/URL to an absolute canonical URL.
func (s Site) Canonical(href string) string {
	return s.AbsoluteURL(href)
}

// OpenGraph returns key/value OG tags for the given meta.
func (s Site) OpenGraph(m Meta) map[string]string {
	title := s.Title(m.Title)
	typ := m.Type
	if typ == "" {
		typ = "website"
	}
	image := m.Image
	if image == "" {
		image = s.DefaultImage
	}
	tags := map[string]string{
		"og:title": title,
		"og:type":  typ,
	}
	if s.SiteName != "" {
		tags["og:site_name"] = s.SiteName
	}
	if desc := s.Description(m.Description); desc != "" {
		tags["og:description"] = desc
	}
	if can := s.Canonical(m.Canonical); can != "" {
		tags["og:url"] = can
	}
	if image != "" {
		tags["og:image"] = s.AbsoluteURL(image)
	}
	return tags
}

// Twitter returns Twitter card meta tags.
func (s Site) Twitter(m Meta) map[string]string {
	card := m.TwitterCard
	if card == "" {
		card = "summary_large_image"
	}
	title := s.Title(m.Title)
	image := m.Image
	if image == "" {
		image = s.DefaultImage
	}
	tags := map[string]string{
		"twitter:card":  card,
		"twitter:title": title,
	}
	if desc := s.Description(m.Description); desc != "" {
		tags["twitter:description"] = desc
	}
	if image != "" {
		tags["twitter:image"] = s.AbsoluteURL(image)
	}
	if s.TwitterHandle != "" {
		handle := s.TwitterHandle
		if !strings.HasPrefix(handle, "@") {
			handle = "@" + handle
		}
		tags["twitter:site"] = handle
	}
	return tags
}

// JSONLD marshals data as a JSON-LD script body (without surrounding tags).
func JSONLD(data any) (string, error) {
	if data == nil {
		return "", nil
	}
	b, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("seo: json-ld: %w", err)
	}
	return string(b), nil
}

// HeadHTML renders a complete set of SEO <meta>/<link>/<script> tags for templates.
func (s Site) HeadHTML(m Meta) (string, error) {
	var b strings.Builder
	title := s.Title(m.Title)
	if title != "" {
		fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(title))
	}
	if desc := s.Description(m.Description); desc != "" {
		fmt.Fprintf(&b, `<meta name="description" content="%s">`+"\n", html.EscapeString(desc))
	}
	if can := s.Canonical(m.Canonical); can != "" {
		fmt.Fprintf(&b, `<link rel="canonical" href="%s">`+"\n", html.EscapeString(can))
	}
	if m.Robots != "" {
		fmt.Fprintf(&b, `<meta name="robots" content="%s">`+"\n", html.EscapeString(m.Robots))
	}
	writeMetaProps(&b, s.OpenGraph(m))
	writeMetaNames(&b, s.Twitter(m))
	ld := m.JSONLD
	if ld == nil && s.SiteName != "" {
		ld = map[string]any{
			"@context": "https://schema.org",
			"@type":    "WebSite",
			"name":     s.SiteName,
			"url":      s.BaseURL,
		}
	}
	if ld != nil {
		raw, err := JSONLD(ld)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, `<script type="application/ld+json">%s</script>`+"\n", raw)
	}
	return b.String(), nil
}

func writeMetaProps(b *strings.Builder, tags map[string]string) {
	keys := sortedKeys(tags)
	for _, k := range keys {
		fmt.Fprintf(b, `<meta property="%s" content="%s">`+"\n", html.EscapeString(k), html.EscapeString(tags[k]))
	}
}

func writeMetaNames(b *strings.Builder, tags map[string]string) {
	keys := sortedKeys(tags)
	for _, k := range keys {
		fmt.Fprintf(b, `<meta name="%s" content="%s">`+"\n", html.EscapeString(k), html.EscapeString(tags[k]))
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func isAbsoluteURL(href string) bool {
	u, err := url.Parse(href)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// JoinURL joins base and a relative path (exported helper for sitemap builders).
func JoinURL(base, p string) string {
	base = strings.TrimRight(base, "/")
	if p == "" || p == "/" {
		return base + "/"
	}
	if isAbsoluteURL(p) {
		return p
	}
	return base + "/" + strings.TrimLeft(path.Clean("/"+p), "/")
}
