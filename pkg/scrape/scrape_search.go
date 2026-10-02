package scrape

import (
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/xbapps/xbvr/pkg/models"
)

// WebSearchResult is one raw web-search hit before scraper matching.
type WebSearchResult struct {
	Title string
	URL   string
}

// ScrapeCandidate is a web-search hit on a host served by a known scraper,
// ranked so the user can pick which source to single-scrape.
type ScrapeCandidate struct {
	ScraperID   string `json:"scraper_id"`
	ScraperName string `json:"scraper_name"`
	Domain      string `json:"domain"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Reason      string `json:"reason"`
	Preferred   bool   `json:"preferred"`
	Studio      bool   `json:"studio"`
}

// preferredCoreDomain ranks above every other source when both match the
// same scene: scenes routinely scraped through SexLikeReal keep SLR
// formatting, even when VRPorn ranks higher in the raw search results.
const preferredCoreDomain = "sexlikereal"

// SearchDuckDuckGo runs a query against DuckDuckGo's lightweight HTML
// endpoint (no API key) and returns the raw result links in rank order.
func SearchDuckDuckGo(query string) ([]WebSearchResult, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", "https://html.duckduckgo.com/html/?q="+url.QueryEscape(query), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return parseDuckDuckGoHTML(resp.Body), nil
}

// parseDuckDuckGoHTML extracts result links from a DDG HTML response.
// It is a pure function over the body so tests can feed it fixtures.
func parseDuckDuckGoHTML(r io.Reader) []WebSearchResult {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil
	}
	var out []WebSearchResult
	doc.Find("a.result__a").Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok || href == "" {
			return
		}
		u := extractResultURL(href)
		if u == "" {
			return
		}
		out = append(out, WebSearchResult{Title: strings.TrimSpace(s.Text()), URL: u})
	})
	return out
}

// extractResultURL unwraps DDG's redirect links (/l/?uddg=<target>) and
// accepts direct https links. Anything else is dropped.
func extractResultURL(href string) string {
	if strings.HasPrefix(href, "//") {
		href = "https:" + href
	}
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if uddg := u.Query().Get("uddg"); uddg != "" {
		return uddg
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	if u.Host == "" {
		return ""
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// RankScrapeCandidates keeps only hits served by a known scraper and ranks
// them: the preferred source first, then raw search order. Hits sharing a
// core domain (e.g. every SLR studio on sexlikereal.com) collapse to one
// candidate using that domain's "*-single_scene" fallback when present,
// which resolves studio attribution from the page itself.
func RankScrapeCandidates(results []WebSearchResult, scrapers []models.Scraper) []ScrapeCandidate {
	cands, _ := rankScrapeCandidates(results, scrapers, nil)
	return cands
}

// RankScrapeCandidatesForSite ranks like RankScrapeCandidates but boosts
// hits on the requested studio's own scraper group (primary plus
// MasterSiteId-linked alternates, e.g. a custom SLR studio with a VRPorn
// alternate, or a RealityLovers-style custom on its own domain with an SLR
// alternate): those candidates sort first and are scraped through the
// studio's own scraper instead of the generic domain fallback. It also
// returns the distinct unmatched core domains, the cluster signal for
// proposing new batch scrapers.
func RankScrapeCandidatesForSite(results []WebSearchResult, scrapers []models.Scraper, site string) ([]ScrapeCandidate, []string) {
	var group map[string]bool
	if strings.TrimSpace(site) != "" {
		group = map[string]bool{}
		for _, s := range ResolveSiteGroup(site, scrapers) {
			group[s.ID] = true
		}
	}
	return rankScrapeCandidates(results, scrapers, group)
}

func rankScrapeCandidates(results []WebSearchResult, scrapers []models.Scraper, group map[string]bool) ([]ScrapeCandidate, []string) {
	seenURL := map[string]bool{}
	seenDomain := map[string]bool{}
	seenUnknown := map[string]bool{}
	var unknown []string
	type ranked struct {
		cand  ScrapeCandidate
		order int
	}
	var out []ranked

	for i, r := range results {
		u, err := url.Parse(r.URL)
		if err != nil || u.Hostname() == "" {
			continue
		}
		if seenURL[r.URL] {
			continue
		}
		core := GetCoreDomain(strings.ToLower(u.Hostname()))

		var matched []models.Scraper
		for _, s := range scrapers {
			if s.Domain == "" {
				continue
			}
			if GetCoreDomain(strings.ToLower(s.Domain)) == core {
				matched = append(matched, s)
			}
		}
		if len(matched) == 0 {
			if !seenUnknown[core] {
				seenUnknown[core] = true
				unknown = append(unknown, core)
			}
			continue
		}
		if seenDomain[core] {
			continue
		}
		seenURL[r.URL] = true
		seenDomain[core] = true

		pick := pickScraperForDomain(matched, group)
		studio := group != nil && group[pick.ID]
		cand := ScrapeCandidate{
			ScraperID:   pick.ID,
			ScraperName: pick.Name,
			Domain:      pick.Domain,
			URL:         r.URL,
			Title:       r.Title,
			Reason:      candidateReason(core, len(matched), studio),
			Preferred:   core == preferredCoreDomain,
			Studio:      studio,
		}
		out = append(out, ranked{cand: cand, order: i})
	}

	sort.SliceStable(out, func(a, b int) bool {
		sa := out[a].cand.Studio
		sb := out[b].cand.Studio
		if sa != sb {
			return sa
		}
		pa := out[a].cand.Preferred
		pb := out[b].cand.Preferred
		if pa != pb {
			return pa
		}
		return out[a].order < out[b].order
	})

	cands := make([]ScrapeCandidate, 0, len(out))
	for _, r := range out {
		cands = append(cands, r.cand)
	}
	return cands, unknown
}

// pickScraperForDomain chooses the scraper to single-scrape a URL on a
// shared domain: a studio-group member when the caller resolved one (the
// studio's own custom scraper beats the generic fallback), else the
// "*-single_scene" fallback when the domain has one (it attributes the
// studio from the page), else the lowest ID for determinism.
func pickScraperForDomain(matched []models.Scraper, group map[string]bool) models.Scraper {
	sorted := append([]models.Scraper(nil), matched...)
	sort.Slice(sorted, func(a, b int) bool { return sorted[a].ID < sorted[b].ID })
	if group != nil {
		for _, s := range sorted {
			if group[s.ID] {
				return s
			}
		}
	}
	for _, s := range sorted {
		if strings.HasSuffix(s.ID, "-single_scene") {
			return s
		}
	}
	return sorted[0]
}

func candidateReason(core string, matchCount int, studio bool) string {
	if studio {
		return "Studio match"
	}
	if core == preferredCoreDomain {
		return "Preferred formatting source (SLR)"
	}
	if matchCount == 1 {
		return "Studio site"
	}
	return "Aggregator"
}

// normalizeSiteName folds a studio or scraper name for comparison.
func normalizeSiteName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ResolveSiteGroup finds the scraper group for a studio name: scrapers
// whose normalized name or ID matches, plus MasterSiteId-linked scrapers
// transitively in both directions, so a primary and its alternates resolve
// together even when they live on different core domains.
func ResolveSiteGroup(site string, scrapers []models.Scraper) []models.Scraper {
	want := normalizeSiteName(site)
	if want == "" {
		return nil
	}
	byID := make(map[string]models.Scraper, len(scrapers))
	for _, s := range scrapers {
		byID[s.ID] = s
	}
	inGroup := map[string]bool{}
	for _, s := range scrapers {
		if normalizeSiteName(s.Name) == want || normalizeSiteName(s.ID) == want {
			inGroup[s.ID] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, s := range scrapers {
			if inGroup[s.ID] {
				continue
			}
			for id := range inGroup {
				m := byID[id]
				if pointsAt(s.MasterSiteId, m) || pointsAt(m.MasterSiteId, s) {
					inGroup[s.ID] = true
					changed = true
					break
				}
			}
		}
	}
	var out []models.Scraper
	for _, s := range scrapers {
		if inGroup[s.ID] {
			out = append(out, s)
		}
	}
	return out
}

// pointsAt reports whether a MasterSiteId link names the target scraper,
// matching against both its ID and its name.
func pointsAt(link string, target models.Scraper) bool {
	n := normalizeSiteName(link)
	if n == "" {
		return false
	}
	return n == normalizeSiteName(target.ID) || n == normalizeSiteName(target.Name)
}

// SearchScrapeCandidates is the live path: DDG search filtered and ranked
// against the registered scrapers.
func SearchScrapeCandidates(query string) ([]ScrapeCandidate, error) {
	results, err := SearchDuckDuckGo(query)
	if err != nil {
		return nil, err
	}
	return RankScrapeCandidates(results, models.GetScrapers()), nil
}

// SearchScrapeCandidatesForSite is the studio-aware live path: same search,
// but hits on the studio's own scraper group sort first with the studio's
// scraper selected, and unmatched domains come back for clustering.
func SearchScrapeCandidatesForSite(site, query string) ([]ScrapeCandidate, []string, error) {
	results, err := SearchDuckDuckGo(query)
	if err != nil {
		return nil, nil, err
	}
	cands, unknown := RankScrapeCandidatesForSite(results, models.GetScrapers(), site)
	return cands, unknown, nil
}

// ComposeSearchQuery builds the web-search query from structured input — a
// site, a scene title, and performer names, as an RSS item provides. Empty
// parts are dropped so sparse items still produce a usable query.
func ComposeSearchQuery(title, site string, performers []string) string {
	parts := make([]string, 0, 2+len(performers))
	for _, p := range append([]string{site, title}, performers...) {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " ")
}
