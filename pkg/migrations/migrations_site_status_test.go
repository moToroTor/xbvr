package migrations

import (
	"path/filepath"
	"testing"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"

	"github.com/xbapps/xbvr/pkg/models"
)

// The run-status columns must exist before runScrapers can persist them;
// on an old sites table the migration adds all five and leaves existing
// rows readable.
func TestMigrate0091SiteScrapeRunStatus(t *testing.T) {
	db, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "migration_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.AutoMigrate(&models.Site{}).Error; err != nil {
		t.Fatal(err)
	}
	for _, col := range []string{
		"last_scrape_started_at", "last_scrape_finished_at", "last_scrape_new_scenes",
		"last_scrape_blocked", "last_scrape_errors",
	} {
		if err := db.Exec("ALTER TABLE sites DROP COLUMN " + col).Error; err != nil {
			t.Fatalf("simulating old schema: %v", err)
		}
	}
	if err := db.Exec(`INSERT INTO sites (id, name) VALUES (?, ?)`, "vrhush", "VRHush").Error; err != nil {
		t.Fatal(err)
	}

	if err := Migrate0091SiteScrapeRunStatus(db); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	var cols []struct {
		Name string `gorm:"column:name"`
	}
	if err := db.Raw("PRAGMA table_info(sites)").Scan(&cols).Error; err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, c := range cols {
		have[c.Name] = true
	}
	for _, col := range []string{
		"last_scrape_started_at", "last_scrape_finished_at", "last_scrape_new_scenes",
		"last_scrape_blocked", "last_scrape_errors",
	} {
		if !have[col] {
			t.Errorf("column %s missing after migration", col)
		}
	}

	var site models.Site
	if err := db.Where("id = ?", "vrhush").First(&site).Error; err != nil {
		t.Fatalf("pre-existing row unreadable after migration: %v", err)
	}
	if site.Name != "VRHush" {
		t.Errorf("site name = %q, want %q", site.Name, "VRHush")
	}
}
