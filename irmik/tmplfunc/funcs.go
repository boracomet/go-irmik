// Package tmplfunc provides shared html/template helpers for Irmik render engines.
// Inspired by StatiGo framework/templates/functions.go (MIT); reimplemented here.
package tmplfunc

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/boracomet/go-irmik/irmik/slug"
)

// Map returns the default FuncMap. Merge into render.Options.Funcs.
func Map() template.FuncMap {
	return template.FuncMap{
		"dict":           Dict,
		"set":            Set,
		"add":            Add,
		"sub":            Sub,
		"div":            Div,
		"mod":            Mod,
		"until":          Until,
		"slugify":        slug.Slugify,
		"safeHTML":       SafeHTML,
		"safeURL":        SafeURL,
		"prettyJSON":     PrettyJSON,
		"formatDate":     FormatDate,
		"formatDateTime": FormatDateTime,
	}
}

// Dict builds a map from alternating key/value pairs (keys must be strings).
func Dict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("dict: odd number of arguments")
	}
	out := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict: key at %d is not a string", i)
		}
		out[key] = values[i+1]
	}
	return out, nil
}

// Set mutates m and returns "" so it can be used as {{set . "k" v}}.
func Set(m map[string]any, key string, value any) string {
	if m != nil {
		m[key] = value
	}
	return ""
}

func Add(a, b int) int { return a + b }
func Sub(a, b int) int { return a - b }

func Div(a, b int) int {
	if b == 0 {
		return 0
	}
	return a / b
}

func Mod(a, b int) int {
	if b == 0 {
		return 0
	}
	return a % b
}

// Until returns [0, n).
func Until(n int) []int {
	if n <= 0 {
		return nil
	}
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func SafeHTML(s string) template.HTML { return template.HTML(s) }
func SafeURL(s string) template.URL   { return template.URL(strings.TrimSpace(s)) }

// PrettyJSON returns indented JSON marked as template.JS (for script tags).
func PrettyJSON(v any) template.JS {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return template.JS("{}")
	}
	return template.JS(b)
}

// FormatDate formats an RFC3339 string (or returns input on parse failure).
// lang "tr" uses Turkish month names; anything else uses English January 2, 2006.
func FormatDate(dateStr, lang string) string {
	if dateStr == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return dateStr
	}
	return FormatDateTime(t, lang)
}

// FormatDateTime formats t for lang.
func FormatDateTime(t time.Time, lang string) string {
	if t.IsZero() {
		return ""
	}
	if strings.EqualFold(lang, "tr") {
		months := [...]string{
			"", "Ocak", "Şubat", "Mart", "Nisan", "Mayıs", "Haziran",
			"Temmuz", "Ağustos", "Eylül", "Ekim", "Kasım", "Aralık",
		}
		return fmt.Sprintf("%d %s %d", t.Day(), months[t.Month()], t.Year())
	}
	return t.Format("January 2, 2006")
}
