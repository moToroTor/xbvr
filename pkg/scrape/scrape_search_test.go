package scrape

import (
	"strings"
	"testing"

	"github.com/xbapps/xbvr/pkg/models"
)

func testScrapers() []models.Scraper {
	return []models.Scraper{
		{ID: "slr-single_scene", Name: "SLR - Other Studios", Domain: "sexlikereal.com"},
		{ID: "vrporn-single_scene", Name: "VRPorn - Other Studios", Domain: "vrporn.com"},
		{ID: "badoinkvr", Name: "BadoinkVR", Domain: "badoinkvr.com"},
	}
}

func TestRankPrefersSLROverVRPorn(t *testing.T) {
	results := []WebSearchResult{
		{Title: "Scene on VRPorn", URL: "https://vrporn.com/scene-x/"},
		{Title: "Scene on SLR", URL: "https://www.sexlikereal.com/scenes/scene-x-123"},
	}
	cands := RankScrapeCandidates(results, testScrapers())
	if len(cands) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(cands))
	}
	if cands[0].Domain != "sexlikereal.com" {
		t.Errorf("expected SLR first, got %q", cands[0].Domain)
	}
	if !cands[0].Preferred {
		t.Error("expected SLR candidate to be marked preferred")
	}
	if cands[0].ScraperID != "slr-single_scene" {
		t.Errorf("expected SLR single-scene fallback, got %q", cands[0].ScraperID)
	}
}

func TestRankDropsUnknownHosts(t *testing.T) {
	results := []WebSearchResult{
		{Title: "TPDB", URL: "https://theporndb.net/scenes/123"},
		{Title: "Badoink", URL: "https://badoinkvr.com/vrpornvideo/scene-y"},
	}
	cands := RankScrapeCandidates(results, testScrapers())
	if len(cands) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(cands))
	}
	if cands[0].ScraperID != "badoinkvr" {
		t.Errorf("expected badoinkvr, got %q", cands[0].ScraperID)
	}
}

func TestRankDedupesRepeatDomains(t *testing.T) {
	results := []WebSearchResult{
		{Title: "A", URL: "https://www.sexlikereal.com/scenes/a-1"},
		{Title: "B", URL: "https://www.sexlikereal.com/scenes/b-2"},
	}
	cands := RankScrapeCandidates(results, testScrapers())
	if len(cands) != 1 {
		t.Fatalf("expected 1 collapsed candidate, got %d", len(cands))
	}
}

func TestExtractResultURLUnwrapsRedirect(t *testing.T) {
	got := extractResultURL("//duckduckgo.com/l/?uddg=https%3A%2F%2Fwww.sexlikereal.com%2Fscenes%2Fa-1&rut=abc")
	if got != "https://www.sexlikereal.com/scenes/a-1" {
		t.Errorf("unexpected unwrap: %q", got)
	}
}

func TestParseDuckDuckGoHTML(t *testing.T) {
	html := `<html><body>
		<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fbadoinkvr.com%2Fvrpornvideo%2Fx&rut=1">Hit One</a>
		<a class="result__a" href="https://example.com/no">Nope</a>
		</body></html>`
	got := parseDuckDuckGoHTML(strings.NewReader(html))
	if len(got) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(got))
	}
	if got[0].URL != "https://badoinkvr.com/vrpornvideo/x" || got[0].Title != "Hit One" {
		t.Errorf("unexpected first hit: %+v", got[0])
	}
}
