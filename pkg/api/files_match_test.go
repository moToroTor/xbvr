package api

import (
	"testing"
)

// A scene with no stored filenames (empty column rather than "[]" or
// "null") must still match: the filenames list starts fresh instead of
// failing the request.
func TestAppendFilenameToArr(t *testing.T) {
	cases := []struct {
		name     string
		stored   string
		filename string
		want     string
	}{
		{"empty starts fresh", "", "a.mp4", `["a.mp4"]`},
		{"null starts fresh", "null", "a.mp4", `["a.mp4"]`},
		{"empty array appends", "[]", "a.mp4", `["a.mp4"]`},
		{"existing appends", `["x.mp4"]`, "a.mp4", `["x.mp4","a.mp4"]`},
		{"corrupt starts fresh", "garbage", "a.mp4", `["a.mp4"]`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := appendFilenameToArr(c.stored, c.filename); got != c.want {
				t.Errorf("appendFilenameToArr(%q, %q) = %q, want %q", c.stored, c.filename, got, c.want)
			}
		})
	}
}
