package scrape

import (
	"testing"

	"github.com/xbapps/xbvr/pkg/models"
)

func TestScraperIDsForHost(t *testing.T) {
	scrapers := []models.Scraper{
		{ID: "tranzvr", Domain: "povr.com"},
		{ID: "brasilvr", Domain: "povr.com"},
		{ID: "badoinkvr", Domain: "badoinkvr.com"},
		{ID: "nodomain"},
	}
	for _, tc := range []struct {
		host string
		want []string
	}{
		{"povr.com", []string{"tranzvr", "brasilvr"}},
		{"www.povr.com", []string{"tranzvr", "brasilvr"}},
		{"POVR.COM", []string{"tranzvr", "brasilvr"}},
		{"badoinkvr.com:443", []string{"badoinkvr"}},
		{"unknown.example.com", nil},
	} {
		got := scraperIDsForHost(scrapers, tc.host)
		if len(got) != len(tc.want) {
			t.Errorf("scraperIDsForHost(%q) = %v, want %v", tc.host, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("scraperIDsForHost(%q) = %v, want %v", tc.host, got, tc.want)
				break
			}
		}
	}
}

// recordVisitError feeds the shared-infrastructure case through the real
// registered scrapers: povr.com serves several scraper IDs and a refusal
// there blocks all of them.
func TestRecordVisitErrorSharedHost(t *testing.T) {
	ids := scraperIDsForHost(models.GetScrapers(), "povr.com")
	if len(ids) < 2 {
		t.Fatalf("expected povr.com to resolve to multiple scrapers, got %v", ids)
	}
	recordVisitError("povr.com", 403)
	for _, id := range ids {
		blocked, failed, detail := TakeRunStats(id)
		if blocked != 1 || failed != 0 {
			t.Errorf("TakeRunStats(%q) = (%d, %d), want (1, 0)", id, blocked, failed)
		}
		if detail != `{"403":1}` {
			t.Errorf("TakeRunStats(%q) detail = %q, want %q", id, detail, `{"403":1}`)
		}
	}
}

func TestRecordVisitErrorUnknownHost(t *testing.T) {
	ids := scraperIDsForHost(models.GetScrapers(), "povr.com")
	if len(ids) == 0 {
		t.Fatal("expected povr.com to resolve to scrapers")
	}
	recordVisitError("no-such-site.invalid", 403)
	for _, id := range ids {
		if blocked, failed, detail := TakeRunStats(id); blocked != 0 || failed != 0 || detail != "" {
			t.Errorf("unknown host leaked into %q stats: (%d, %d, %q)", id, blocked, failed, detail)
		}
	}
}

func TestTakeRunStatsDrains(t *testing.T) {
	ids := scraperIDsForHost(models.GetScrapers(), "povr.com")
	if len(ids) == 0 {
		t.Fatal("expected povr.com to resolve to scrapers")
	}
	id := ids[0]
	recordVisitError("povr.com", 500)
	blocked, failed, detail := TakeRunStats(id)
	if blocked != 0 || failed != 1 {
		t.Fatalf("TakeRunStats = (%d, %d), want (0, 1)", blocked, failed)
	}
	if detail != `{"500":1}` {
		t.Errorf("TakeRunStats detail = %q, want %q", detail, `{"500":1}`)
	}
	if blocked, failed, detail := TakeRunStats(id); blocked != 0 || failed != 0 || detail != "" {
		t.Errorf("second TakeRunStats = (%d, %d, %q), want (0, 0, \"\")", blocked, failed, detail)
	}
	if blocked, failed, detail := TakeRunStats("never-ran"); blocked != 0 || failed != 0 || detail != "" {
		t.Errorf("TakeRunStats(never-ran) = (%d, %d, %q), want (0, 0, \"\")", blocked, failed, detail)
	}
}

// The breakdown accumulates per status across visits, including status 0
// (no response) as "timeout", and drains with the counters.
func TestTakeRunStatsBreakdown(t *testing.T) {
	ids := scraperIDsForHost(models.GetScrapers(), "povr.com")
	if len(ids) == 0 {
		t.Fatal("expected povr.com to resolve to scrapers")
	}
	id := ids[0]
	recordVisitError("povr.com", 403)
	recordVisitError("povr.com", 403)
	recordVisitError("povr.com", 502)
	recordVisitError("povr.com", 0)
	blocked, failed, detail := TakeRunStats(id)
	if blocked != 2 || failed != 2 {
		t.Fatalf("TakeRunStats = (%d, %d), want (2, 2)", blocked, failed)
	}
	// encoding/json sorts map keys, so the expectation is deterministic.
	if want := `{"403":2,"502":1,"timeout":1}`; detail != want {
		t.Errorf("TakeRunStats detail = %q, want %q", detail, want)
	}
	for _, other := range ids[1:] {
		TakeRunStats(other)
	}
}
