// Package utils holds small helpers with no project dependencies.
package utils

import "strings"

// Slug turns text into a URL-safe form: lowercase letters and digits, joined by
// hyphens. "Hello, World!" becomes "hello-world".
func Slug(text string) string {
	var b strings.Builder

	gap := false

	for _, r := range strings.ToLower(text) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if gap && b.Len() > 0 {
				b.WriteByte('-')
			}

			b.WriteRune(r)

			gap = false

			continue
		}

		gap = true
	}

	return b.String()
}
