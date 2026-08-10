package cache

import (
	"fmt"
	"strings"
	"time"
)

// Key builds an ISR/page cache key from method, path, and locale.
// Example: GET|/blog/hello|en
func Key(method, path, locale string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = "GET"
	}
	path = strings.TrimSpace(path)
	if path == "" {
		path = "/"
	}
	locale = strings.TrimSpace(locale)
	if locale == "" {
		locale = "en"
	}
	return fmt.Sprintf("%s|%s|%s", method, path, locale)
}

func unixNano(n int64) time.Time {
	return time.Unix(0, n)
}
