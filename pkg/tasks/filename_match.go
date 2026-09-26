package tasks

import (
	"bytes"
	"encoding/json"
	"strings"
)

// htmlEscape mirrors the historical query encoding used when matching files
// to scenes (json.HTMLEscape: & -> \u0026, < -> \u003c, > -> \u003e).
func htmlEscapeFilename(s string) string {
	var buf bytes.Buffer
	json.HTMLEscape(&buf, []byte(s))
	return buf.String()
}

// escapeLike quotes LIKE metacharacters so a filename matches literally.
// Must be paired with `ESCAPE '!'` in the SQL. `!` is used instead of the
// usual backslash because backslash is a string-literal escape in
// MySQL/MariaDB, so `ESCAPE '\'` is a syntax error (Error 1064) there —
// it only ever worked on sqlite.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `!`, `!!`)
	s = strings.ReplaceAll(s, `%`, `!%`)
	s = strings.ReplaceAll(s, `_`, `!_`)
	return s
}

// extSwaps covers sidecar files stored under a scene's filenames.
var filenameExtSwaps = [][2]string{
	{".funscript", ".mp4"},
	{".hsp", ".mp4"},
	{".srt", ".mp4"},
	{".cmscript", ".mp4"},
}

// filenameMatchVariants returns every stored form a scene-file basename may
// appear in, for LIKE matching against JSON columns (filenames_arr,
// external_data).
//
// Both the raw basename and its HTML-escaped form are returned because rows
// written by older versions store the raw form (e.g. `A & B.mp4`) while
// current writes store the escaped form (`A \u0026 B.mp4`). Querying only
// the escaped form can never match raw rows — and on MySQL the backslash
// in \u0026 is itself a LIKE escape, so the escaped-only query fails there
// even against escaped rows (xbapps/xbvr#1739).
func filenameMatchVariants(base string) []string {
	variants := []string{base}
	if esc := htmlEscapeFilename(base); esc != base {
		variants = append(variants, esc)
	}
	var out []string
	for _, v := range variants {
		out = append(out, v)
		for _, sw := range filenameExtSwaps {
			out = append(out, strings.Replace(v, sw[0], sw[1], -1))
		}
	}
	return out
}
