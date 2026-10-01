package scrape

import (
	"errors"
	"testing"

	"github.com/xbapps/xbvr/pkg/models"
)

func fullJAVRScene() models.ScrapedScene {
	return models.ScrapedScene{
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
		want  int
	}{
		{"empty", models.ScrapedScene{}, 0},
		{"title only", models.ScrapedScene{Title: "T"}, 3},
		{"full", fullJAVRScene(), 12},
		{"no title or covers", models.ScrapedScene{Cast: []string{"A"}, Duration: 60, Released: "2024-01-01", Synopsis: "S", Studio: "St"}, 6},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ScoreScrapedScene(&c.scene); got != c.want {
				t.Errorf("ScoreScrapedScene = %d, want %d", got, c.want)
			}
		})
	}
	if got := ScoreScrapedScene(nil); got != 0 {
		t.Errorf("ScoreScrapedScene(nil) = %d, want 0", got)
	}
}

func TestIsCompleteScrapedScene(t *testing.T) {
	full := fullJAVRScene()
	if !IsCompleteScrapedScene(&full) {
		t.Error("full scene should be complete")
	}
	partial := full
	partial.Duration = 0
	if IsCompleteScrapedScene(&partial) {
		t.Error("scene without duration should not be complete")
	}
	if IsCompleteScrapedScene(nil) {
		t.Error("nil should not be complete")
	}
	if IsCompleteScrapedScene(&models.ScrapedScene{}) {
		t.Error("empty scene should not be complete")
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
