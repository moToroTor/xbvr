package migrations

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"

	"github.com/xbapps/xbvr/pkg/models"
)

// The ProjectionOverride field shipped without a migration, so File saves on
// pre-existing databases failed with "no such column" and could kill the
// process. This simulates the old schema (full files table minus the new
// column), proves the save fails, then drives the real migration.
func TestMigrate0089FileProjectionOverride(t *testing.T) {
	db, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "migration_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.AutoMigrate(&models.File{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ALTER TABLE files DROP COLUMN projection_override").Error; err != nil {
		t.Fatalf("simulating old schema: %v", err)
	}
	if err := db.Exec("INSERT INTO files (filename) VALUES ('old-scene.mp4')").Error; err != nil {
		t.Fatal(err)
	}

	// authentic failure on the old schema
	if err := db.Create(&models.File{Filename: "doomed.mp4"}).Error; err == nil {
		t.Fatal("expected File save to fail on old schema, it succeeded")
	} else if !strings.Contains(err.Error(), "projection_override") {
		t.Fatalf("unexpected save error on old schema: %v", err)
	}

	if err := Migrate0089FileProjectionOverride(db); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	var cols []struct {
		Name string `gorm:"column:name"`
	}
	if err := db.Raw("PRAGMA table_info(files)").Scan(&cols).Error; err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range cols {
		if c.Name == "projection_override" {
			found = true
		}
	}
	if !found {
		t.Fatal("projection_override column missing after migration")
	}

	// the pre-existing row survives and the new field round-trips
	f := models.File{Filename: "new-scene.mp4", ProjectionOverride: "180"}
	if err := db.Create(&f).Error; err != nil {
		t.Fatalf("saving File after migration: %v", err)
	}
	var got models.File
	if err := db.Where("filename = ?", "new-scene.mp4").First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.ProjectionOverride != "180" {
		t.Errorf("ProjectionOverride = %q, want %q", got.ProjectionOverride, "180")
	}
	var count int
	if err := db.Model(&models.File{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("row count = %d, want 2 (old row preserved)", count)
	}
}
