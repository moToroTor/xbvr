package models

import (
	"path/filepath"
	"testing"
	"time"
)

// pre0091Site mirrors the sites table before migration 0091 added the
// run-status columns (last_scrape_*).
type pre0091Site struct {
	ID             string `gorm:"primary_key"`
	Name           string
	AvatarURL      string
	IsBuiltin      bool
	IsEnabled      bool
	LastUpdate     time.Time
	Subscribed     bool
	LimitScraping  bool
	MasterSiteID   string
	MatchingParams string `gorm:"size:1000"`
	ScrapeStash    bool
}

func (pre0091Site) TableName() string { return "sites" }

// initSiteRecord must upsert using only long-standing columns. At startup it
// races the background schema migrations, and a full-struct Site write
// fatals with "no such column" on a database that has not reached 0091 yet
// (killed dogfood, whose DB was at 0089).
func TestInitSiteRecordPre0091Schema(t *testing.T) {
	db := openTestDB(t, filepath.Join(t.TempDir(), "sites.db"))
	if err := db.AutoMigrate(&pre0091Site{}).Error; err != nil {
		t.Fatalf("old schema: %v", err)
	}

	scraper := Scraper{ID: "vrhush", Name: "VRHush", AvatarURL: "http://x/y.jpg"}

	// Create path on the old schema.
	if err := initSiteRecord(db, scraper); err != nil {
		t.Fatalf("create on pre-0091 schema: %v", err)
	}
	var st Site
	if err := db.Where("id = ?", "vrhush").First(&st).Error; err != nil {
		t.Fatalf("read back: %v", err)
	}
	if st.Name != "VRHush" || st.AvatarURL != "http://x/y.jpg" || !st.IsBuiltin {
		t.Errorf("row = %+v, want managed fields set", st)
	}

	// A site-managed flag outside InitSites' remit must survive the update.
	if err := db.Model(&Site{}).Where("id = ?", "vrhush").UpdateColumn("subscribed", true).Error; err != nil {
		t.Fatalf("seed unmanaged field: %v", err)
	}

	// Update path on the old schema.
	scraper.Name = "VRHush Renamed"
	if err := initSiteRecord(db, scraper); err != nil {
		t.Fatalf("update on pre-0091 schema: %v", err)
	}
	if err := db.Where("id = ?", "vrhush").First(&st).Error; err != nil {
		t.Fatalf("read back: %v", err)
	}
	if st.Name != "VRHush Renamed" {
		t.Errorf("name = %q, want renamed", st.Name)
	}
	if !st.Subscribed {
		t.Error("update clobbered unmanaged subscribed flag")
	}
}

// The normal path on a current schema keeps working.
func TestInitSiteRecordCurrentSchema(t *testing.T) {
	db := openTestDB(t, filepath.Join(t.TempDir(), "sites.db"))
	if err := db.AutoMigrate(&Site{}).Error; err != nil {
		t.Fatalf("current schema: %v", err)
	}
	if err := initSiteRecord(db, Scraper{ID: "vrhush", Name: "VRHush"}); err != nil {
		t.Fatalf("current schema: %v", err)
	}
}
