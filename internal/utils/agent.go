package utils

import (
	"net/http"
	"strings"
	"unicode"
)

const (
	maxUserAgentLength   = 512 // RFC 7230 recommends 8K but we enforce sane limits
	sanitizedReplacement = '?'
)

func GetUserAgent(r *http.Request) string {
	// Get raw header value
	ua := strings.TrimSpace(r.Header.Get("User-Agent"))

	// Truncate to prevent abuse
	if len(ua) > maxUserAgentLength {
		ua = ua[:maxUserAgentLength]
	}

	// Sanitize potentially dangerous characters
	return strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) && !unicode.IsControl(r) {
			return r
		}
		return sanitizedReplacement
	}, ua)
}
