package migrations

import (
	"path/filepath"
	"testing"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"

	"github.com/xbapps/xbvr/pkg/models"
)

// The error-detail column must exist before runScrapers can persist it;
// on a post-0091 sites table the migration adds just it and leaves
// existing rows readable.
func TestMigrate0092SiteScrapeErrorDetail(t *testing.T) {
	db, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "migration_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.AutoMigrate(&models.Site{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ALTER TABLE sites DROP COLUMN last_scrape_error_detail").Error; err != nil {
		t.Fatalf("simulating pre-0092 schema: %v", err)
	}
	if err := db.Exec(`INSERT INTO sites (id, name) VALUES (?, ?)`, "vrhush", "VRHush").Error; err != nil {
		t.Fatal(err)
	}

	if err := Migrate0092SiteScrapeErrorDetail(db); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	var cols []struct {
		Name string `gorm:"column:name"`
	}
	if err := db.Raw("PRAGMA table_info(sites)").Scan(&cols).Error; err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range cols {
		if c.Name == "last_scrape_error_detail" {
			found = true
		}
	}
	if !found {
		t.Error("column last_scrape_error_detail missing after migration")
	}

	var site models.Site
	if err := db.Where("id = ?", "vrhush").First(&site).Error; err != nil {
		t.Fatalf("pre-existing row unreadable after migration: %v", err)
	}
	site.LastScrapeErrorDetail = `{"403":12}`
	if err := db.Save(&site).Error; err != nil {
		t.Fatalf("detail not persistable after migration: %v", err)
	}
}
