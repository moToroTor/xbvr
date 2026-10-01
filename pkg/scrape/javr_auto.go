package scrape

import (
	"github.com/xbapps/xbvr/pkg/models"
)

// JAVREngine adapts one JAV lookup engine to a uniform fetch shape so the
// auto-picker can try them in order. The func type absorbs signature
// differences (R18D returns error, the rest return void) and makes the
// picker unit-testable without network.
type JAVREngine struct {
	Name  string
	Fetch func(out *[]models.ScrapedScene, query string) error
}

// DefaultJAVREngines returns the JAV engines in priority order: javdatabase
// first, R18D last (DVD-IDs only resolve for scenes released on r18.com
// before 2022-06, as the UI already notes).
func DefaultJAVREngines() []JAVREngine {
	return []JAVREngine{
		{Name: "javdatabase", Fetch: func(out *[]models.ScrapedScene, q string) error {
			ScrapeJavDB(out, q)
			return nil
		}},
		{Name: "javlibrary", Fetch: func(out *[]models.ScrapedScene, q string) error {
			ScrapeJavLibrary(out, q)
			return nil
		}},
		{Name: "javland", Fetch: func(out *[]models.ScrapedScene, q string) error {
			ScrapeJavLand(out, q)
			return nil
		}},
		{Name: "r18d", Fetch: func(out *[]models.ScrapedScene, q string) error {
			return ScrapeR18D(out, q)
		}},
	}
}

// ScoreScrapedScene rates a scraped scene's completeness (max 12). Weights
// are a starting assumption — tune after seeing real engine output.
func ScoreScrapedScene(s *models.ScrapedScene) int {
	if s == nil {
		return 0
	}
	score := 0
	if s.Title != "" {
		score += 3
	}
	if len(s.Covers) > 0 {
		score += 3
	}
	if len(s.Cast) > 0 {
		score += 2
	}
	if s.Duration > 0 {
		score++
	}
	if s.Released != "" {
		score++
	}
	if s.Synopsis != "" {
		score++
	}
	if s.Studio != "" {
		score++
	}
	return score
}

// IsCompleteScrapedScene reports whether a scene has the core fields that
// make further engine attempts unnecessary.
func IsCompleteScrapedScene(s *models.ScrapedScene) bool {
	return s != nil && s.Title != "" && len(s.Covers) > 0 && len(s.Cast) > 0 && s.Duration > 0
}

// PickBestJAVR queries engines in order and returns the highest-scoring
// scene, stopping early once the best so far is complete. An engine that
// errors, panics, or returns nothing scores 0 and is skipped. pause, when
// non-nil, runs between attempts (IP-block politeness); tests pass nil.
// Returns nil when no engine produced a scene.
func PickBestJAVR(query string, engines []JAVREngine, pause func()) *models.ScrapedScene {
	var best *models.ScrapedScene
	bestScore := -1

	for i, e := range engines {
		if i > 0 && pause != nil {
			pause()
		}
		got := runJAVREngine(e, query)
		if len(got) == 0 {
			log.Infof("JAV auto: %s scored 0 (no scenes)", e.Name)
			continue
		}
		var top models.ScrapedScene
		topScore := -1
		for _, s := range got {
			if sc := ScoreScrapedScene(&s); sc > topScore {
				top, topScore = s, sc
			}
		}
		score := topScore
		log.Infof("JAV auto: %s scored %d", e.Name, score)
		if score > bestScore {
			c := top
			best, bestScore = &c, score
		}
		if IsCompleteScrapedScene(best) {
			break
		}
	}

	if best != nil {
		log.Infof("JAV auto: winner scored %d", bestScore)
	}
	return best
}

// runJAVREngine runs one engine, normalizing error/panic/empty to an empty
// result. Live scrapers can panic on bad page content (see #2279), and one
// engine must never kill the auto lookup.
func runJAVREngine(e JAVREngine, query string) (out []models.ScrapedScene) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("JAV auto: %s panicked and was skipped: %v", e.Name, r)
			out = nil
		}
	}()
	var scenes []models.ScrapedScene
	if err := e.Fetch(&scenes, query); err != nil {
		log.Errorf("JAV auto: %s failed: %v", e.Name, err)
		return nil
	}
	return scenes
}
