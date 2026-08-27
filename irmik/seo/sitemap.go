package seo

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
	"time"
)

// URLEntry is one sitemap URL row.
type URLEntry struct {
	Loc         string
	LastMod     *time.Time
	ChangeFreq  string  // always|hourly|daily|weekly|monthly|yearly|never
	Priority    float64 // 0.0–1.0; zero means omit unless SetPriority is used via PrioritySet
	prioritySet bool
}

// WithPriority returns a copy with Priority set (including 0.0).
func (e URLEntry) WithPriority(p float64) URLEntry {
	e.Priority = p
	e.prioritySet = true
	return e
}

type urlset struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc        string  `xml:"loc"`
	LastMod    string  `xml:"lastmod,omitempty"`
	ChangeFreq string  `xml:"changefreq,omitempty"`
	Priority   *string `xml:"priority,omitempty"`
}

// SitemapXML builds a sitemap.xml document from URL entries.
// Loc values should already be absolute; relative paths are joined with baseURL when baseURL is non-empty.
func SitemapXML(baseURL string, entries []URLEntry) ([]byte, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	seen := make(map[string]struct{}, len(entries))
	urls := make([]sitemapURL, 0, len(entries))
	for _, e := range entries {
		loc := strings.TrimSpace(e.Loc)
		if loc == "" {
			continue
		}
		if !isAbsoluteURL(loc) {
			if baseURL == "" {
				return nil, fmt.Errorf("seo: sitemap entry %q needs baseURL", loc)
			}
			loc = JoinURL(baseURL, loc)
		}
		if _, ok := seen[loc]; ok {
			continue
		}
		seen[loc] = struct{}{}
		row := sitemapURL{Loc: loc, ChangeFreq: e.ChangeFreq}
		if e.LastMod != nil && !e.LastMod.IsZero() {
			row.LastMod = e.LastMod.UTC().Format("2006-01-02")
		}
		if e.prioritySet {
			p := fmt.Sprintf("%.1f", e.Priority)
			row.Priority = &p
		}
		urls = append(urls, row)
	}
	sort.Slice(urls, func(i, j int) bool { return urls[i].Loc < urls[j].Loc })
	doc := urlset{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}
	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("seo: marshal sitemap: %w", err)
	}
	return append([]byte(xml.Header), out...), nil
}

// SitemapFromPaths builds a sitemap from site-relative route paths.
func SitemapFromPaths(baseURL string, paths []string) ([]byte, error) {
	entries := make([]URLEntry, 0, len(paths))
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		entries = append(entries, URLEntry{Loc: p})
	}
	return SitemapXML(baseURL, entries)
}

// RobotsTxt returns a minimal robots.txt that allows all crawlers and points at sitemap.xml.
// When generateSitemap is false, the Sitemap line is omitted.
func RobotsTxt(baseURL string, generateSitemap bool) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	var b strings.Builder
	b.WriteString("User-agent: *\n")
	b.WriteString("Allow: /\n")
	if generateSitemap && baseURL != "" {
		b.WriteString("\nSitemap: ")
		b.WriteString(baseURL)
		b.WriteString("/sitemap.xml\n")
	}
	return b.String()
}

// RobotsFromConfig is a convenience wrapper using SEOConfig.GenerateSitemap.
func RobotsFromConfig(baseURL string, generateSitemap bool) string {
	return RobotsTxt(baseURL, generateSitemap)
}
