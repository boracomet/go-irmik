// Package paginate parses list query params (page, per_page, sort, order, q)
// with clamped limits and SQL-friendly Offset/Limit plus whitelist OrderBy.
// Opt-in helper for admin tables and REST list endpoints.
package paginate

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Params holds normalized list query values.
type Params struct {
	Page    int
	PerPage int
	Sort    string // requested sort key (may be empty)
	Order   string // "asc" or "desc"
	Q       string // free-text search (caller applies)
}

// Options configures Parse defaults and limits.
type Options struct {
	DefaultPage    int // default 1
	DefaultPerPage int // default 20
	MaxPerPage     int // default 100
	DefaultSort    string
	DefaultOrder   string   // "asc" or "desc"; default "asc"
	SortWhitelist  []string // allowed sort keys (case-sensitive); empty = accept DefaultSort only when set
}

// Parse reads page/per_page/sort/order/q from the query string.
func Parse(c *gin.Context, opts Options) Params {
	if opts.DefaultPage <= 0 {
		opts.DefaultPage = 1
	}
	if opts.DefaultPerPage <= 0 {
		opts.DefaultPerPage = 20
	}
	if opts.MaxPerPage <= 0 {
		opts.MaxPerPage = 100
	}
	order := strings.ToLower(strings.TrimSpace(opts.DefaultOrder))
	if order != "desc" {
		order = "asc"
	}

	page := opts.DefaultPage
	if v := c.Query("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}

	perPage := opts.DefaultPerPage
	if v := c.Query("per_page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			perPage = n
		}
	}
	if perPage > opts.MaxPerPage {
		perPage = opts.MaxPerPage
	}

	sortKey := strings.TrimSpace(c.Query("sort"))
	if sortKey == "" {
		sortKey = opts.DefaultSort
	}
	if len(opts.SortWhitelist) > 0 {
		if !contains(opts.SortWhitelist, sortKey) {
			sortKey = opts.DefaultSort
			if !contains(opts.SortWhitelist, sortKey) {
				sortKey = ""
			}
		}
	}

	if v := strings.ToLower(strings.TrimSpace(c.Query("order"))); v == "asc" || v == "desc" {
		order = v
	}

	return Params{
		Page:    page,
		PerPage: perPage,
		Sort:    sortKey,
		Order:   order,
		Q:       strings.TrimSpace(c.Query("q")),
	}
}

// Offset is SQL OFFSET for the current page.
func (p Params) Offset() int {
	if p.Page < 1 {
		return 0
	}
	return (p.Page - 1) * p.Limit()
}

// Limit is SQL LIMIT (PerPage).
func (p Params) Limit() int {
	if p.PerPage < 1 {
		return 20
	}
	return p.PerPage
}

// OrderBy returns a safe "column ASC|DESC" fragment using columns whitelist.
// columns maps sort keys → SQL column identifiers (never interpolate user input as columns).
// Returns empty string when Sort is empty or not in the whitelist.
func (p Params) OrderBy(columns map[string]string) string {
	if p.Sort == "" || columns == nil {
		return ""
	}
	col, ok := columns[p.Sort]
	if !ok || col == "" || !safeIdent(col) {
		return ""
	}
	dir := "ASC"
	if strings.EqualFold(p.Order, "desc") {
		dir = "DESC"
	}
	return col + " " + dir
}

// OrderByWhitelist is OrderBy with a flat whitelist where key == SQL column.
func (p Params) OrderByWhitelist(allowed []string) string {
	m := make(map[string]string, len(allowed))
	for _, a := range allowed {
		m[a] = a
	}
	return p.OrderBy(m)
}

// FilterColumn returns column if it is in the whitelist; otherwise "".
// Use for dynamic WHERE column selection (never concatenate unsanitized input).
func FilterColumn(name string, whitelist []string) string {
	name = strings.TrimSpace(name)
	if name == "" || !contains(whitelist, name) {
		return ""
	}
	if !safeIdent(name) {
		return ""
	}
	return name
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// safeIdent allows simple SQL identifiers: letters, digits, underscore, optional schema.dot.
func safeIdent(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		case r == '.':
			if i == 0 || i == len(s)-1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
