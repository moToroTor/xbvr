package tasks

import (
	"path/filepath"
	"testing"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/xbapps/xbvr/pkg/models"
)

func TestNormalizeDupTitleParity(t *testing.T) {
	// Pairs that must normalize identically (real cross-scraper dupes).
	pairs := [][2]string{
		{"3Some: Lillyy In Sedona", "3some: Lillyy in Sedona"},
		{"Millie's First Show, She Goes Crazy", "Millie's First Show, She Goes Crazy"},
		{"1-800-BANG-ME", "1-800-BANG-ME"},
		{"Tom & Jerry", "Tom and Jerry"},
	}
	for _, p := range pairs {
		if a, b := NormalizeDupTitle(p[0]), NormalizeDupTitle(p[1]); a != b {
			t.Errorf("NormalizeDupTitle(%q)=%q != NormalizeDupTitle(%q)=%q", p[0], a, p[1], b)
		}
	}
	if NormalizeDupTitle("Tom & Jerry") != NormalizeDupTitle("Tom + Jerry") {
		t.Error("+/& should normalize identically (relink matcher rule)")
	}
}

func TestGroupDupRows(t *testing.T) {
	rows := []dupRow{
		{ID: 1, SceneID: "slr-1", ScraperID: "sitea", Title: "Same Title Here", Duration: 60},
		{ID: 2, SceneID: "b-1", ScraperID: "siteb", Title: "same title here!", Duration: 60},
		{ID: 3, SceneID: "b-2", ScraperID: "siteb", Title: "Same Title Here", Duration: 61},
		{ID: 4, SceneID: "a-2", ScraperID: "sitea", Title: "Same Title Here", Duration: 60},
		{ID: 5, SceneID: "a-3", ScraperID: "sitea", Title: "", Duration: 60},
		{ID: 6, SceneID: "a-4", ScraperID: "sitea", Title: "No Duration", Duration: 0},
		{ID: 7, SceneID: "c-1", ScraperID: "sitec", Title: "Lonely", Duration: 10},
	}
	groups := groupDupRows(rows)
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	if len(groups[0]) != 3 {
		t.Fatalf("group has %d rows, want 3 (IDs 1,2,4)", len(groups[0]))
	}
	seen := map[uint]bool{}
	for _, r := range groups[0] {
		seen[r.ID] = true
	}
	for _, id := range []uint{1, 2, 4} {
		if !seen[id] {
			t.Errorf("group missing row %d", id)
		}
	}
}

func dupesTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "dupes.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	for _, v := range []interface{}{
		&models.Scene{}, &models.Actor{}, &models.ExternalReference{},
		&models.ExternalReferenceLink{}, &models.KV{}, &models.File{},
	} {
		if err := db.AutoMigrate(v).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestFindLinkDismissDuplicates(t *testing.T) {
	db := dupesTestDB(t)
	mk := func(sceneID, scraper, site, title string, dur int) uint {
		s := models.Scene{SceneID: sceneID, ScraperId: scraper, Site: site, Title: title, Duration: dur}
		if err := db.Create(&s).Error; err != nil {
			t.Fatal(err)
		}
		return s.ID
	}
	a := mk("slr-40297", "jackandjillvr", "JackandJillVR", "Millie's First Show, She Goes Crazy", 61)
	b := mk("vrporn-x", "jackandjill-vrporn", "JackandJill VR (VRPorn)", "Millie's First Show, She Goes Crazy", 61)
	_ = mk("other-1", "siteb", "Other", "Millie's First Show, She Goes Crazy", 62)

	groups, err := FindDuplicateGroups(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || len(groups[0].Scenes) != 2 {
		t.Fatalf("got %+v, want one 2-scene group", groups)
	}

	// Suggestion prefers the scene with files.
	if err := db.Create(&models.File{SceneID: b, Filename: "x.mp4"}).Error; err != nil {
		t.Fatal(err)
	}
	groups, _ = FindDuplicateGroups(db)
	if groups[0].SuggestedKeep != b || groups[0].SuggestWhy != "has matched files" {
		t.Errorf("suggestion = (%d, %q), want (%d, has matched files)", groups[0].SuggestedKeep, groups[0].SuggestWhy, b)
	}

	// Link hides the group and records a manual (99999) link.
	if err := LinkDuplicate(db, b, a); err != nil {
		t.Fatal(err)
	}
	var links []models.ExternalReferenceLink
	db.Where("internal_db_id = ? AND match_type = 99999", b).Find(&links)
	if len(links) != 1 {
		t.Fatalf("got %d manual links, want 1", len(links))
	}
	groups, _ = FindDuplicateGroups(db)
	if len(groups) != 0 {
		t.Errorf("linked pair still reported: %+v", groups)
	}

	// Dismiss hides a fresh group instead.
	d := mk("slr-9", "jackandjillvr", "JackandJillVR", "Another Shared Title", 30)
	e := mk("vrporn-9", "jackandjill-vrporn", "JackandJill VR (VRPorn)", "Another Shared Title!", 30)
	groups, _ = FindDuplicateGroups(db)
	if len(groups) != 1 {
		t.Fatalf("fresh group missing: %+v", groups)
	}
	if err := DismissDuplicate(db, d, e); err != nil {
		t.Fatal(err)
	}
	groups, _ = FindDuplicateGroups(db)
	if len(groups) != 0 {
		t.Errorf("dismissed pair still reported: %+v", groups)
	}
}
