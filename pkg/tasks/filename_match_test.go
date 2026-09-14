package tasks

import (
	"strings"
	"testing"
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
	if got := escapeLike(`100%_x\y`); got != `100\%\_x\\y` {
		t.Errorf("escapeLike = %q, want %q", got, `100\%\_x\\y`)
	}
}
