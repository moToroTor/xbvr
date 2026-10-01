package scrape

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

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
// before 2022-06, as the UI already notes). The DVD-ID engines get the
// query in DVD form (a fanza content ID like kavr00403 is converted);
// R18D takes either form (it falls back internally), so it gets the raw
// query. Single-engine lookups bypass this and are unchanged.
func DefaultJAVREngines() []JAVREngine {
	return []JAVREngine{
		{Name: "javdatabase", Fetch: func(out *[]models.ScrapedScene, q string) error {
			ScrapeJavDB(out, ToDVDID(q))
			return nil
		}},
		{Name: "javlibrary", Fetch: func(out *[]models.ScrapedScene, q string) error {
			ScrapeJavLibrary(out, ToDVDID(q))
			return nil
		}},
		{Name: "javland", Fetch: func(out *[]models.ScrapedScene, q string) error {
			ScrapeJavLand(out, ToDVDID(q))
			return nil
		}},
		{Name: "r18d", Fetch: func(out *[]models.ScrapedScene, q string) error {
			return ScrapeR18D(out, q)
		}},
	}
}

// ToDVDID converts a fanza content ID (kavr00403) to its DVD form
// (KAVR-403). Anything already containing a dash, or not shaped like
// letters-followed-by-digits, passes through untouched.
func ToDVDID(query string) string {
	q := strings.TrimSpace(query)
	if q == "" || strings.Contains(q, "-") {
		return query
	}
	i := 0
	for i < len(q) && (q[i] < '0' || q[i] > '9') && ((q[i] >= 'a' && q[i] <= 'z') || (q[i] >= 'A' && q[i] <= 'Z')) {
		i++
	}
	if i == 0 || i == len(q) {
		return query
	}
	// Strip a "1" studio prefix that ToFanzaContentID adds for 3dsvr/fsdss.
	letters := q[:i]
	if len(letters) > 1 && letters[0] == '1' {
		if rest := letters[1:]; rest == "3dsvr" || rest == "fsdss" {
			letters = rest
		}
	}
	n, err := strconv.Atoi(q[i:])
	if err != nil {
		return query
	}
	return strings.ToUpper(letters) + "-" + strconv.Itoa(n)
}

// ToFanzaContentID converts a DVD ID (KAVR-403) to its fanza content-ID
// form (kavr00403), mirroring the fallback guess in determineContentId.
// Anything else passes through untouched.
func ToFanzaContentID(query string) string {
	parts := strings.Split(strings.TrimSpace(query), "-")
	if len(parts) != 2 {
		return query
	}
	site := strings.ToLower(strings.TrimSpace(parts[0]))
	num, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || site == "" {
		return query
	}
	if site == "3dsvr" || site == "fsdss" {
		site = "1" + site
	}
	return fmt.Sprintf("%s%05d", site, num)
}

// normalizeCode reduces a title or code to lowercase alphanumerics so
// "KAVR-403", "kavr403" and "kavr00403" compare by content, not format.
func normalizeCode(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isCodeTitle reports whether a title is just the lookup code. Every JAV
// engine sets Title to the DVD ID (or r18d to the content ID) when the
// real title is unavailable, so such a title must not earn title points.
func isCodeTitle(title, query string) bool {
	t := normalizeCode(title)
	if t == "" {
		return false
	}
	for _, q := range strings.Split(query, ",") {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		if t == normalizeCode(q) || t == normalizeCode(ToDVDID(q)) || t == normalizeCode(ToFanzaContentID(q)) {
			return true
		}
	}
	return false
}

// HasRealTitle reports whether a scene has a genuine title rather than the
// lookup code echoed back by the engine.
func HasRealTitle(s *models.ScrapedScene, query string) bool {
	return s != nil && s.Title != "" && !isCodeTitle(s.Title, query)
}

// ScoreScrapedScene rates a scraped scene's completeness (max 12). Weights
// are a starting assumption — tune after seeing real engine output. A
// title that merely echoes the lookup code scores nothing.
func ScoreScrapedScene(s *models.ScrapedScene, query string) int {
	if s == nil {
		return 0
	}
	score := 0
	if HasRealTitle(s, query) {
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
// make further engine attempts unnecessary. A code echo is not a title.
func IsCompleteScrapedScene(s *models.ScrapedScene, query string) bool {
	return HasRealTitle(s, query) && len(s.Covers) > 0 && len(s.Cast) > 0 && s.Duration > 0
}

// JAVREngineResult is one engine's contribution to a lookup: its
// top-scoring scene (nil when the engine produced nothing) plus the
// scores the compare view renders.
type JAVREngineResult struct {
	Engine   string               `json:"engine"`
	Score    int                  `json:"score"`
	Complete bool                 `json:"complete"`
	Scene    *models.ScrapedScene `json:"scene"`
}

// sceneMatchesQuery reports whether a scraped scene is actually the queried
// code. Engines can return near-misses (observed live: r18d's DVD lookup
// answered KAVR-403 with KAVR-043); persisting those mislabels the library,
// so mismatches are discarded before scoring. Either ID field may carry the
// code, in DVD or fanza form.
func sceneMatchesQuery(s *models.ScrapedScene, query string) bool {
	want := map[string]bool{}
	for _, q := range strings.Split(query, ",") {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		want[normalizeCode(q)] = true
		want[normalizeCode(ToDVDID(q))] = true
		want[normalizeCode(ToFanzaContentID(q))] = true
	}
	return want[normalizeCode(s.SceneID)] || want[normalizeCode(s.SiteID)]
}

// attemptJAVREngine runs one engine and scores its best scene. Shared by
// the auto-pick loop (early stop) and the compare view (all engines).
func attemptJAVREngine(e JAVREngine, query string) JAVREngineResult {
	res := JAVREngineResult{Engine: e.Name}
	got := runJAVREngine(e, query)
	matched := make([]models.ScrapedScene, 0, len(got))
	for _, s := range got {
		if sceneMatchesQuery(&s, query) {
			matched = append(matched, s)
		}
	}
	if len(matched) == 0 {
		if len(got) > 0 {
			log.Infof("JAV auto: %s returned %d scene(s) for a different code, skipped", e.Name, len(got))
		} else {
			log.Infof("JAV auto: %s scored 0 (no scenes)", e.Name)
		}
		return res
	}
	got = matched
	var top models.ScrapedScene
	topScore := -1
	for _, s := range got {
		if sc := ScoreScrapedScene(&s, query); sc > topScore {
			top, topScore = s, sc
		}
	}
	c := top
	res.Scene, res.Score = &c, topScore
	res.Complete = IsCompleteScrapedScene(&c, query)
	log.Infof("JAV auto: %s scored %d", e.Name, topScore)
	return res
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
		res := attemptJAVREngine(e, query)
		if res.Scene == nil {
			continue
		}
		if res.Score > bestScore {
			best, bestScore = res.Scene, res.Score
		}
		if res.Complete && res.Score >= bestScore {
			break
		}
	}

	if best != nil {
		log.Infof("JAV auto: winner scored %d", bestScore)
	}
	return best
}

// CompareJAVR runs every engine concurrently and returns each engine's
// scored top scene for the compare view. No pacing is needed between
// attempts: each engine hits a different host, so unlike batch codes
// against one site (issue #398) there is nothing to be polite to, and no
// early stop — the point is seeing them all.
func CompareJAVR(query string, engines []JAVREngine) []JAVREngineResult {
	out := make([]JAVREngineResult, len(engines))
	var wg sync.WaitGroup
	for i, e := range engines {
		wg.Add(1)
		go func(i int, e JAVREngine) {
			defer wg.Done()
			out[i] = attemptJAVREngine(e, query)
		}(i, e)
	}
	wg.Wait()
	return out
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
