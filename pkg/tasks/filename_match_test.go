package tasks

import (
	"database/sql"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// xbapps/xbvr#1739: files with "&" never rematch after a move+rescan.
func TestMatchAmpersand(t *testing.T) {
	variants := filenameMatchVariants("A & B.mp4")

	has := func(want string) bool {
		for _, v := range variants {
			if v == want {
				return true
			}
		}
		return false
	}

	// Both stored forms must be queried: current writes escape & as \u0026,
	// legacy rows store it raw.
	if !has(`A & B.mp4`) {
		t.Errorf("missing raw variant in %q", variants)
	}
	if !has(`A \u0026 B.mp4`) {
		t.Errorf("missing escaped variant in %q", variants)
	}
}

func TestFilenameMatchVariantsPlain(t *testing.T) {
	variants := filenameMatchVariants("movie.mp4")
	if len(variants) == 0 || variants[0] != "movie.mp4" {
		t.Fatalf("first variant must be the raw basename, got %q", variants)
	}
	for _, v := range variants {
		if strings.Contains(v, `\u`) {
			t.Fatalf("plain name must not gain escaped variants: %q", variants)
		}
	}
}

func TestFilenameMatchVariantsSidecar(t *testing.T) {
	variants := filenameMatchVariants("scene.funscript")
	found := false
	for _, v := range variants {
		if v == "scene.mp4" {
			found = true
		}
	}
	if !found {
		t.Errorf("sidecar swap missing in %q", variants)
	}
}

func TestEscapeLike(t *testing.T) {
	if got := escapeLike(`100%_x!y\z`); got != `100!%!_x!!y\z` {
		t.Errorf("escapeLike = %q, want %q", got, `100!%!_x!!y\z`)
	}
}

// The ESCAPE clause must avoid backslash: `ESCAPE '\'` is a syntax error
// (Error 1064) on MySQL/MariaDB, where backslash escapes the string
// literal itself. `!` is literal-safe on sqlite, MySQL and MariaDB —
// verified here end to end against in-memory sqlite, including a
// backslash (HTML-escaped rows like `A \u0026 B.mp4` must match as-is).
func TestEscapeLikeRoundTrip(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE t (v TEXT)`); err != nil {
		t.Fatal(err)
	}
	rows := []string{
		`["BaDoinkVR_Read_Between_the_Cheeks_7k_180_180x180_3dh_LR.mp4"]`,
		`["100%_x!!y.mp4"]`,
		`["A \u0026 B.mp4"]`,
	}
	for _, r := range rows {
		if _, err := db.Exec(`INSERT INTO t (v) VALUES (?)`, r); err != nil {
			t.Fatal(err)
		}
	}

	match := func(pattern string) int {
		var n int
		err := db.QueryRow(`SELECT COUNT(*) FROM t WHERE v LIKE ? ESCAPE '!'`, `%`+escapeLike(pattern)+`%`).Scan(&n)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	for file, want := range map[string]int{
		`BaDoinkVR_Read_Between_the_Cheeks_7k_180_180x180_3dh_LR.mp4`: 1,
		`100%_x!!y.mp4`:  1,
		`A & B.mp4`:      0, // raw form is a different stored row, not a wildcard hit
		`A \u0026 B.mp4`: 1,
		`%.mp4`:          0, // % must not act as a wildcard
		`_`:              2, // _ matches literal underscores, not every row
	} {
		if got := match(file); got != want {
			t.Errorf("match(%q) = %d, want %d", file, got, want)
		}
	}
}
