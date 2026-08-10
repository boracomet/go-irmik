// Package slug converts titles and paths into URL-safe slugs.
// Unicode folding follows StatiGo's utils.Slugify idea (MIT); Turkish
// character map matches common TR web conventions used in StatiGo templates.
package slug

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// turkishFold maps Turkish letters before lowercasing / NFD stripping.
// Dotless ı must be handled before ToLower (İ→i, I→ı in Turkish locale is OS-dependent).
var turkishFold = strings.NewReplacer(
	"ı", "i", "İ", "i",
	"ğ", "g", "Ğ", "g",
	"ü", "u", "Ü", "u",
	"ş", "s", "Ş", "s",
	"ö", "o", "Ö", "o",
	"ç", "c", "Ç", "c",
)

// Slugify returns a lowercase URL slug for s.
func Slugify(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = turkishFold.Replace(s)
	s = strings.ToLower(s)

	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	if folded, _, err := transform.String(t, s); err == nil {
		s = folded
	}

	s = nonAlnum.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
