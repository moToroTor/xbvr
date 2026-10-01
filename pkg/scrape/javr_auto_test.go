package scrape

import (
	"errors"
	"testing"

	"github.com/xbapps/xbvr/pkg/models"
)

func fullJAVRScene() models.ScrapedScene {
	return models.ScrapedScene{
		SceneID:  "CODE-1",
		SiteID:   "CODE-1",
		Title:    "Scene Title",
		Covers:   []string{"https://example.com/cover.jpg"},
		Cast:     []string{"Actor One"},
		Duration: 120,
		Released: "2024-01-02",
		Synopsis: "A synopsis.",
		Studio:   "Studio",
	}
}

func TestScoreScrapedScene(t *testing.T) {
	cases := []struct {
		name  string
		scene models.ScrapedScene
		query string
		want  int
	}{
		{"empty", models.ScrapedScene{}, "OTHER-999", 0},
		{"title only", models.ScrapedScene{Title: "T"}, "OTHER-999", 3},
		{"full", fullJAVRScene(), "OTHER-999", 12},
		{"no title or covers", models.ScrapedScene{Cast: []string{"A"}, Duration: 60, Released: "2024-01-01", Synopsis: "S", Studio: "St"}, "OTHER-999", 6},
		{"dvd code as title scores nothing", models.ScrapedScene{Title: "KAVR-403"}, "KAVR-403", 0},
		{"fanza code as title scores nothing", models.ScrapedScene{Title: "kavr00403"}, "KAVR-403", 0},
		{"dashless code as title scores nothing", models.ScrapedScene{Title: "kavr403"}, "KAVR-403", 0},
		{"real title with code query scores", models.ScrapedScene{Title: "A Real Title"}, "KAVR-403", 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ScoreScrapedScene(&c.scene, c.query); got != c.want {
				t.Errorf("ScoreScrapedScene = %d, want %d", got, c.want)
			}
		})
	}
	if got := ScoreScrapedScene(nil, "OTHER-999"); got != 0 {
		t.Errorf("ScoreScrapedScene(nil) = %d, want 0", got)
	}
}

func TestIsCompleteScrapedScene(t *testing.T) {
	full := fullJAVRScene()
	if !IsCompleteScrapedScene(&full, "OTHER-999") {
		t.Error("full scene should be complete")
	}
	partial := full
	partial.Duration = 0
	if IsCompleteScrapedScene(&partial, "OTHER-999") {
		t.Error("scene without duration should not be complete")
	}
	codeTitle := full
	codeTitle.Title = "KAVR-403"
	if IsCompleteScrapedScene(&codeTitle, "KAVR-403") {
		t.Error("code echo should not count as a title")
	}
	if IsCompleteScrapedScene(nil, "OTHER-999") {
		t.Error("nil should not be complete")
	}
	if IsCompleteScrapedScene(&models.ScrapedScene{}, "OTHER-999") {
		t.Error("empty scene should not be complete")
	}
}

func TestJAVRCodeConversion(t *testing.T) {
	dvdCases := map[string]string{
		"KAVR-403":    "KAVR-403",
		"kavr00403":   "KAVR-403",
		"84vrkm00139": "84vrkm00139", // leading digits: ambiguous, untouched
		"VRKM-139":    "VRKM-139",
		"nonsense":    "nonsense",
		"":            "",
	}
	for in, want := range dvdCases {
		if got := ToDVDID(in); got != want {
			t.Errorf("ToDVDID(%q) = %q, want %q", in, got, want)
		}
	}
	fanzaCases := map[string]string{
		"KAVR-403":  "kavr00403",
		"VRKM-139":  "vrkm00139",
		"3DSVR-878": "13dsvr00878",
		"kavr00403": "kavr00403",
		"nonsense":  "nonsense",
	}
	for in, want := range fanzaCases {
		if got := ToFanzaContentID(in); got != want {
			t.Errorf("ToFanzaContentID(%q) = %q, want %q", in, got, want)
		}
	}
}

func stubEngine(name string, scenes []models.ScrapedScene, err error, calls *int) JAVREngine {
	return JAVREngine{Name: name, Fetch: func(out *[]models.ScrapedScene, q string) error {
		*calls++
		*out = append(*out, scenes...)
		return err
	}}
}

func TestPickBestJAVR(t *testing.T) {
	full, partial := fullJAVRScene(), fullJAVRScene()
	partial.Covers, partial.Synopsis, partial.Studio = nil, "", ""

	t.Run("best wins regardless of order", func(t *testing.T) {
		var a, b int
		engines := []JAVREngine{
			stubEngine("first", []models.ScrapedScene{partial}, nil, &a),
			stubEngine("second", []models.ScrapedScene{full}, nil, &b),
		}
		got := PickBestJAVR("CODE-1", engines, nil)
		if got == nil || got.Synopsis == "" {
			t.Fatalf("expected the full scene to win, got %+v", got)
		}
	})

	t.Run("tie breaks by engine order", func(t *testing.T) {
		other := full
		other.Title = "Other Title"
		var a, b int
		engines := []JAVREngine{
			stubEngine("first", []models.ScrapedScene{full}, nil, &a),
			stubEngine("second", []models.ScrapedScene{other}, nil, &b),
		}
		got := PickBestJAVR("CODE-1", engines, nil)
		if got == nil || got.Title != full.Title {
			t.Fatalf("expected first engine to win ties, got %+v", got)
		}
		if b != 0 {
			t.Errorf("expected early stop after complete winner, second engine called %d times", b)
		}
	})

	t.Run("all empty returns nil", func(t *testing.T) {
		var a int
		if got := PickBestJAVR("CODE-1", []JAVREngine{stubEngine("e", nil, nil, &a)}, nil); got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})

	t.Run("engine error and panic tolerated", func(t *testing.T) {
		var a, b, c int
		engines := []JAVREngine{
			stubEngine("bad", nil, errors.New("boom"), &a),
			{Name: "panicky", Fetch: func(out *[]models.ScrapedScene, q string) error {
				c++
				panic("bad page")
			}},
			stubEngine("good", []models.ScrapedScene{partial}, nil, &b),
		}
		got := PickBestJAVR("CODE-1", engines, nil)
		if got == nil || got.Title != partial.Title {
			t.Fatalf("expected surviving engine result, got %+v", got)
		}
	})

	t.Run("code echo loses to real title", func(t *testing.T) {
		echo, real := fullJAVRScene(), fullJAVRScene()
		echo.SceneID, echo.SiteID = "KAVR-403", "KAVR-403"
		real.SceneID, real.SiteID = "KAVR-403", "KAVR-403"
		echo.Title, echo.HomepageURL = "KAVR-403", "https://www.dmm.co.jp/digital/videoa/-/detail/=/cid=kavr00403/"
		real.HomepageURL = echo.HomepageURL
		var a, b int
		engines := []JAVREngine{
			stubEngine("first", []models.ScrapedScene{echo}, nil, &a),
			stubEngine("second", []models.ScrapedScene{real}, nil, &b),
		}
		got := PickBestJAVR("KAVR-403", engines, nil)
		if got == nil || got.Title != real.Title {
			t.Fatalf("expected real title to win, got %+v", got)
		}
		if b != 1 {
			t.Errorf("expected no early stop on a code echo, second engine called %d times", b)
		}
	})

	t.Run("compare runs every engine with scores", func(t *testing.T) {
		full, partial := fullJAVRScene(), fullJAVRScene()
		partial.Covers, partial.Synopsis, partial.Studio = nil, "", ""
		var a, b, c int
		engines := []JAVREngine{
			stubEngine("first", []models.ScrapedScene{full}, nil, &a),
			stubEngine("second", nil, nil, &b),
			stubEngine("third", []models.ScrapedScene{partial}, nil, &c),
		}
		got := CompareJAVR("CODE-1", engines)
		if len(got) != 3 {
			t.Fatalf("expected 3 results, got %d", len(got))
		}
		if a != 1 || b != 1 || c != 1 {
			t.Errorf("expected every engine attempted once, got %d/%d/%d", a, b, c)
		}
		if got[0].Score != 12 || !got[0].Complete || got[0].Scene == nil {
			t.Errorf("unexpected first result: %+v", got[0])
		}
		if got[1].Scene != nil || got[1].Score != 0 {
			t.Errorf("empty engine should score 0 with nil scene: %+v", got[1])
		}
		if got[2].Score != 7 || got[2].Complete {
			t.Errorf("unexpected third result: %+v", got[2])
		}
	})

	t.Run("wrong-code scene is discarded", func(t *testing.T) {
		wrong := fullJAVRScene()
		wrong.SceneID, wrong.SiteID = "KAVR-043", "KAVR-043"
		var a int
		got := PickBestJAVR("KAVR-403", []JAVREngine{stubEngine("r18d", []models.ScrapedScene{wrong}, nil, &a)}, nil)
		if got != nil {
			t.Errorf("expected near-miss to be discarded, got %+v", got)
		}
	})

	t.Run("fanza-form id matches dvd query", func(t *testing.T) {
		s := fullJAVRScene()
		s.SceneID, s.SiteID = "kavr00403", "kavr00403"
		var a int
		got := PickBestJAVR("KAVR-403", []JAVREngine{stubEngine("r18d", []models.ScrapedScene{s}, nil, &a)}, nil)
		if got == nil || got.SceneID != "kavr00403" {
			t.Errorf("expected fanza-form match to be kept, got %+v", got)
		}
	})

	t.Run("pause runs between attempts", func(t *testing.T) {
		var a, b, pauses int
		engines := []JAVREngine{
			stubEngine("first", []models.ScrapedScene{partial}, nil, &a),
			stubEngine("second", []models.ScrapedScene{partial}, nil, &b),
		}
		PickBestJAVR("CODE-1", engines, func() { pauses++ })
		if pauses != 1 {
			t.Errorf("expected 1 pause between 2 attempts, got %d", pauses)
		}
	})
}
