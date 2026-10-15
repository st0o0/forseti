// Package redact removes credentials and tokens from strings before they are
// logged or returned in errors.
package redact

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// Placeholder replaces redacted values.
const Placeholder = "REDACTED"

// maxTextRunes bounds the length of sanitized free text.
const maxTextRunes = 256

var urlPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)

// URL removes userinfo and replaces every query value with REDACTED, keeping
// scheme, host and path. Unparseable input is returned with any query string
// replaced.
func URL(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		if i := strings.IndexByte(s, '?'); i >= 0 {
			return s[:i] + "?" + Placeholder
		}
		return s
	}
	u.User = nil
	if u.RawQuery != "" {
		q := u.Query()
		parts := make([]string, 0, len(q))
		for k := range q {
			parts = append(parts, url.QueryEscape(k)+"="+Placeholder)
		}
		sort.Strings(parts)
		u.RawQuery = strings.Join(parts, "&")
	}
	u.Fragment = ""
	return u.String()
}

// Text redacts every http(s) URL inside s, collapses whitespace runs
// containing newlines into a single space and truncates the result.
func Text(s string) string {
	s = urlPattern.ReplaceAllStringFunc(s, URL)
	s = strings.Join(strings.Fields(strings.ReplaceAll(s, "\r", " ")), " ")
	return Truncate(s, maxTextRunes)
}

// Truncate shortens s to at most n runes, appending "..." when cut.
func Truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
