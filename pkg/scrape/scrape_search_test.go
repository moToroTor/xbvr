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

func siteGroupScrapers() []models.Scraper {
	return []models.Scraper{
		{ID: "slr-single_scene", Name: "SLR - Other Studios", Domain: "sexlikereal.com"},
		{ID: "vrporn-single_scene", Name: "VRPorn - Other Studios", Domain: "vrporn.com"},
		{ID: "badoinkvr", Name: "BadoinkVR", Domain: "badoinkvr.com"},
		// Anal Delight: custom SLR primary with a custom VRPorn alternate.
		{ID: "analdelight", Name: "Anal Delight", Domain: "sexlikereal.com"},
		{ID: "analdelight-vrporn", Name: "Anal Delight", Domain: "vrporn.com", MasterSiteId: "analdelight"},
		// RealityLovers: custom primary on its own domain, SLR alternate.
		{ID: "realitylovers", Name: "RealityLovers", Domain: "realitylovers.com"},
		{ID: "realitylovers-slr", Name: "RealityLovers", Domain: "sexlikereal.com", MasterSiteId: "realitylovers"},
	}
}

func groupIDs(g []models.Scraper) map[string]bool {
	out := map[string]bool{}
	for _, s := range g {
		out[s.ID] = true
	}
	return out
}

func TestResolveSiteGroup(t *testing.T) {
	all := siteGroupScrapers()

	ad := groupIDs(ResolveSiteGroup("Anal Delight", all))
	if len(ad) != 2 || !ad["analdelight"] || !ad["analdelight-vrporn"] {
		t.Errorf("Anal Delight group = %v, want primary + vrporn alternate", ad)
	}

	// Alternate links the other way too: seed by alternate name still
	// finds the primary (both share the name here); seed by distinct ID.
	rl := groupIDs(ResolveSiteGroup("realitylovers-slr", all))
	if len(rl) != 2 || !rl["realitylovers"] || !rl["realitylovers-slr"] {
		t.Errorf("RealityLovers group from alternate ID = %v, want both", rl)
	}

	if g := ResolveSiteGroup("No Such Studio", all); len(g) != 0 {
		t.Errorf("unknown studio group = %v, want empty", groupIDs(g))
	}
	if g := ResolveSiteGroup("", all); len(g) != 0 {
		t.Errorf("empty site group = %v, want empty", groupIDs(g))
	}
	// Name matching ignores case and punctuation.
	if g := ResolveSiteGroup("anal delight!", all); len(g) != 2 {
		t.Errorf("unnormalized name group size = %d, want 2", len(g))
	}
}

func TestRankForSiteBoostsStudioGroup(t *testing.T) {
	all := siteGroupScrapers()
	results := []WebSearchResult{
		{Title: "VRPorn copy", URL: "https://vrporn.com/anal-scene/"},
		{Title: "SLR copy", URL: "https://www.sexlikereal.com/scenes/anal-scene/"},
		{Title: "Unknown host", URL: "https://newstudio.example/x"},
		{Title: "Unknown host again", URL: "https://newstudio.example/y"},
		{Title: "Badoink", URL: "https://badoinkvr.com/vrpornvideo/x/"},
	}
	cands, unknown := RankScrapeCandidatesForSite(results, all, "Anal Delight")

	if len(cands) != 3 {
		t.Fatalf("candidates = %d, want 3 (unknown host dropped)", len(cands))
	}
	// Studio group first, SLR formatting still preferred within it, and
	// scraped through the studio's own customs — not the generic fallbacks.
	if cands[0].ScraperID != "analdelight" || !cands[0].Studio {
		t.Errorf("first = %+v, want SLR primary studio pick", cands[0])
	}
	if cands[1].ScraperID != "analdelight-vrporn" || !cands[1].Studio {
		t.Errorf("second = %+v, want vrporn alternate studio pick", cands[1])
	}
	if cands[2].ScraperID != "badoinkvr" || cands[2].Studio {
		t.Errorf("third = %+v, want plain badoinkvr, not studio", cands[2])
	}
	if len(unknown) != 1 || unknown[0] != "newstudio" {
		t.Errorf("unknown = %v, want deduped [newstudio]", unknown)
	}
}

func TestRankForSitePrefersAlternateOverFallback(t *testing.T) {
	all := siteGroupScrapers()
	results := []WebSearchResult{
		{Title: "RL main site", URL: "https://realitylovers.com/video/y"},
		{Title: "RL SLR copy", URL: "https://www.sexlikereal.com/scenes/y/"},
	}
	cands, _ := RankScrapeCandidatesForSite(results, all, "RealityLovers")
	if len(cands) != 2 {
		t.Fatalf("candidates = %d, want 2", len(cands))
	}
	// SLR-alternate hit first (studio + preferred formatting), scraped
	// through the studio's alternate, not the generic slr-single_scene
	// fallback; main-site hit second through the studio custom.
	if cands[0].ScraperID != "realitylovers-slr" || !cands[0].Studio {
		t.Errorf("first = %+v, want realitylovers-slr studio pick", cands[0])
	}
	if cands[1].ScraperID != "realitylovers" || !cands[1].Studio {
		t.Errorf("second = %+v, want realitylovers studio pick", cands[1])
	}
}

func TestPerformerPagesSortLast(t *testing.T) {
	all := siteGroupScrapers()
	results := []WebSearchResult{
		{Title: "Performer page", URL: "https://www.sexlikereal.com/pornstars/ariela-donovan-3125"},
		{Title: "Scene page", URL: "https://www.sexlikereal.com/scenes/ariela-donovan-85236"},
		{Title: "VRPorn scene", URL: "https://vrporn.com/scene-z/"},
	}
	cands := RankScrapeCandidates(results, all)
	if len(cands) != 2 {
		t.Fatalf("candidates = %d, want 2 (SLR domain collapses)", len(cands))
	}
	// The SLR scene wins its domain; the VRPorn scene beats the SLR
	// performer page because a performer page can never single-scrape
	// to a scene, even on the preferred source.
	if cands[0].URL != "https://www.sexlikereal.com/scenes/ariela-donovan-85236" {
		t.Errorf("first = %q, want the SLR scene page", cands[0].URL)
	}
	if cands[1].ScraperID != "vrporn-single_scene" {
		t.Errorf("second = %+v, want the VRPorn scene over the SLR performer page", cands[1])
	}
}

func TestScopedSceneQuery(t *testing.T) {
	all := siteGroupScrapers()
	var ad []models.Scraper
	for _, s := range all {
		if s.ID == "analdelight" || s.ID == "analdelight-vrporn" {
			ad = append(ad, s)
		}
	}
	if got := scopedSceneQuery(ad, "Anal Delight Massage"); got != "site:sexlikereal.com inurl:scenes Anal Delight Massage" {
		t.Errorf("scoped = %q, want SLR scene restriction", got)
	}
	if got := scopedSceneQuery([]models.Scraper{{ID: "badoinkvr", Domain: "badoinkvr.com"}}, "BadoinkVR Massage"); got != "" {
		t.Errorf("scoped = %q, want empty without an SLR domain", got)
	}
	if got := scopedSceneQuery(nil, "Anything"); got != "" {
		t.Errorf("scoped = %q, want empty for nil group", got)
	}
}

func TestComposeSearchQuery(t *testing.T) {
	cases := []struct {
		name       string
		title      string
		site       string
		performers []string
		want       string
	}{
		{"full", "Great Scene", "SLR", []string{"Jane Doe", "Ann Lee"}, "SLR Great Scene Jane Doe Ann Lee"},
		{"no performers", "Great Scene", "SLR", nil, "SLR Great Scene"},
		{"no site", "Great Scene", "", []string{"Jane"}, "Great Scene Jane"},
		{"title only", "Great Scene", "", nil, "Great Scene"},
		{"blanks dropped", "  Great Scene ", " ", []string{"", " Jane "}, "Great Scene Jane"},
		{"all empty", "", "", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ComposeSearchQuery(c.title, c.site, c.performers); got != c.want {
				t.Errorf("ComposeSearchQuery = %q, want %q", got, c.want)
			}
		})
	}
}
