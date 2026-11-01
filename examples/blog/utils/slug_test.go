package utils

import "testing"

func TestSlug(t *testing.T) {
	for text, want := range map[string]string{
		"Hello, World!":   "hello-world",
		"  many   gaps  ": "many-gaps",
		"Go 1.27":         "go-1-27",
		"!!!":             "",
	} {
		if got := Slug(text); got != want {
			t.Errorf("Slug(%q) = %q, want %q", text, got, want)
		}
	}
}
