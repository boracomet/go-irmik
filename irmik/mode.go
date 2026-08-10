package irmik

import "github.com/boracomet/go-irmik/irmik/meta"

// Mode selects how a route is rendered.
type Mode = meta.Mode

const (
	ModeSSR    = meta.ModeSSR
	ModeSSG    = meta.ModeSSG
	ModeISR    = meta.ModeISR
	ModeStatic = meta.ModeStatic
	ModeCSR    = meta.ModeCSR
)

// PageMeta configures route-level rendering behavior.
type PageMeta = meta.PageMeta

// DefaultMeta returns SSR with sitemap enabled.
func DefaultMeta() PageMeta {
	return meta.Default()
}
