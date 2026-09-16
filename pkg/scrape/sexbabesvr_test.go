package scrape

import (
	"testing"

	"github.com/mozillazg/go-slugify"
)

func TestSexBabesVRSceneIDFromURL(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{"slug", "https://sexbabesvr.com/video/wet-college-student-remastered/", "wet-college-student-remastered"},
		{"no trailing slash", "https://sexbabesvr.com/video/best-international-fuck-moments", "best-international-fuck-moments"},
		{"query stripped", "https://sexbabesvr.com/video/wet-college-student-remastered/?page=2", "wet-college-student-remastered"},
		{"empty path", "https://sexbabesvr.com/", ""},
	}

	for _, tc := range cases {
		if got := sexBabesVRSceneIDFromURL(tc.url); got != tc.want {
			t.Errorf("%s: sexBabesVRSceneIDFromURL(%q) = %q, want %q", tc.name, tc.url, got, tc.want)
		}
	}

	// Two distinct scene URLs must yield distinct SceneIDs (issue #1867:
	// poster-derived IDs collided, so every scrape overwrote a single row).
	a := slugify.Slugify("SexBabesVR") + "-" + sexBabesVRSceneIDFromURL("https://sexbabesvr.com/video/wet-college-student-remastered/")
	b := slugify.Slugify("SexBabesVR") + "-" + sexBabesVRSceneIDFromURL("https://sexbabesvr.com/video/best-international-fuck-moments/")
	if a == b {
		t.Errorf("SceneID collision: %q == %q", a, b)
	}
}
