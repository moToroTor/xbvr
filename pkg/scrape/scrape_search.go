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
	seenURL := map[string]bool{}
	seenDomain := map[string]bool{}
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
		if len(matched) == 0 || seenDomain[core] {
			continue
		}
		seenURL[r.URL] = true
		seenDomain[core] = true

		pick := pickScraperForDomain(matched)
		cand := ScrapeCandidate{
			ScraperID:   pick.ID,
			ScraperName: pick.Name,
			Domain:      pick.Domain,
			URL:         r.URL,
			Title:       r.Title,
			Reason:      candidateReason(core, len(matched)),
			Preferred:   core == preferredCoreDomain,
		}
		out = append(out, ranked{cand: cand, order: i})
	}

	sort.SliceStable(out, func(a, b int) bool {
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
	return cands
}

// pickScraperForDomain chooses the scraper to single-scrape a URL on a
// shared domain: the "*-single_scene" fallback when the domain has one
// (it attributes the studio from the page), else the lowest ID for
// determinism.
func pickScraperForDomain(matched []models.Scraper) models.Scraper {
	sorted := append([]models.Scraper(nil), matched...)
	sort.Slice(sorted, func(a, b int) bool { return sorted[a].ID < sorted[b].ID })
	for _, s := range sorted {
		if strings.HasSuffix(s.ID, "-single_scene") {
			return s
		}
	}
	return sorted[0]
}

func candidateReason(core string, matchCount int) string {
	if core == preferredCoreDomain {
		return "Preferred formatting source (SLR)"
	}
	if matchCount == 1 {
		return "Studio site"
	}
	return "Aggregator"
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
