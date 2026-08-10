// Package meta holds page rendering mode and metadata shared by router and app.
package meta

import "time"

// Mode selects how a route is rendered.
type Mode string

const (
	ModeSSR    Mode = "ssr"    // render on every request
	ModeSSG    Mode = "ssg"    // render at build time
	ModeISR    Mode = "isr"    // SSG + TTL revalidation
	ModeStatic Mode = "static" // pure static file, no runtime render
	ModeCSR    Mode = "csr"    // shell + client islands only
)

// PageMeta configures route-level rendering behavior.
type PageMeta struct {
	Mode       Mode
	Revalidate time.Duration // ISR only
	Canonical  string
	Robots     string
	Sitemap    bool
	NoIndex    bool
}

// Default returns SSR with sitemap enabled.
func Default() PageMeta {
	return PageMeta{Mode: ModeSSR, Sitemap: true}
}
