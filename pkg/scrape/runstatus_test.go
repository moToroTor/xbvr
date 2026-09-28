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
		blocked, failed := TakeRunStats(id)
		if blocked != 1 || failed != 0 {
			t.Errorf("TakeRunStats(%q) = (%d, %d), want (1, 0)", id, blocked, failed)
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
		if blocked, failed := TakeRunStats(id); blocked != 0 || failed != 0 {
			t.Errorf("unknown host leaked into %q stats: (%d, %d)", id, blocked, failed)
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
	blocked, failed := TakeRunStats(id)
	if blocked != 0 || failed != 1 {
		t.Fatalf("TakeRunStats = (%d, %d), want (0, 1)", blocked, failed)
	}
	if blocked, failed := TakeRunStats(id); blocked != 0 || failed != 0 {
		t.Errorf("second TakeRunStats = (%d, %d), want (0, 0)", blocked, failed)
	}
	if blocked, failed := TakeRunStats("never-ran"); blocked != 0 || failed != 0 {
		t.Errorf("TakeRunStats(never-ran) = (%d, %d), want (0, 0)", blocked, failed)
	}
}
