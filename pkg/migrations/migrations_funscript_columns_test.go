package migrations

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"

	"github.com/xbapps/xbvr/pkg/models"
)

// Reproduces the fatal upgrade crash: an old-schema scenes table (no
// funscript_speed column, i.e. any database whose migrations table
// predates the funscript-speed migrations) cannot persist a full Scene
// struct, which is what migration 0084 does via scene.Save().
func TestFullSceneSaveFailsOnOldSchema(t *testing.T) {
	db, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "migration_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.AutoMigrate(&models.Scene{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ALTER TABLE scenes DROP COLUMN funscript_speed").Error; err != nil {
		t.Fatalf("simulating old schema: %v", err)
	}

	s := models.Scene{SceneID: "naughtyamerica-vr-test", TrailerType: "heresphere"}
	// INSERT omits the default-tagged field, so the row goes in fine.
	if err := db.Save(&s).Error; err != nil {
		t.Fatalf("inserting row on old schema: %v", err)
	}
	// UPDATE writes every struct field — this is what 0084 does to the
	// rows it finds, and it dies on the missing column.
	s.TrailerSource = `{"SceneUrl":"https://example.com/trailer"}`
	if err := db.Save(&s).Error; err == nil {
		t.Fatal("expected full Scene save to fail on old schema, it succeeded")
	} else if !strings.Contains(err.Error(), "funscript_speed") {
		t.Fatalf("unexpected save error on old schema: %v", err)
	}
}

// The ensure-migration must create both columns on the old schema, after
// which 0084's write path (full Scene save of trailer fields) succeeds.
func TestEnsureFunscriptSpeedColumns(t *testing.T) {
	db, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "migration_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.AutoMigrate(&models.Scene{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.File{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ALTER TABLE scenes DROP COLUMN funscript_speed").Error; err != nil {
		t.Fatalf("simulating old schema: %v", err)
	}
	if err := db.Exec("ALTER TABLE files DROP COLUMN funscript_speed").Error; err != nil {
		t.Fatalf("simulating old schema: %v", err)
	}
	if err := db.Exec(
		`INSERT INTO scenes (scene_id, trailer_type, trailer_source) VALUES (?, ?, ?)`,
		"naughtyamerica-vr-test", "heresphere", "https://www.example.com/old",
	).Error; err != nil {
		t.Fatal(err)
	}

	if err := Migrate0000EnsureFunscriptSpeedColumns(db); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	for _, table := range []string{"scenes", "files"} {
		var cols []struct {
			Name string `gorm:"column:name"`
		}
		if err := db.Raw("PRAGMA table_info(" + table + ")").Scan(&cols).Error; err != nil {
			t.Fatal(err)
		}
		found := false
		for _, c := range cols {
			if c.Name == "funscript_speed" {
				found = true
			}
		}
		if !found {
			t.Fatalf("funscript_speed column missing on %s after migration", table)
		}
	}

	// 0084's exact write on the pre-existing row, via the same gorm Save
	// statement that died in production (scene.Save wraps db.Save).
	var s models.Scene
	if err := db.Where("scene_id = ?", "naughtyamerica-vr-test").First(&s).Error; err != nil {
		t.Fatal(err)
	}
	s.TrailerType = "heresphere"
	s.TrailerSource = `{"SceneUrl":"https://api.naughtyapi.com/heresphere/test"}`
	if err := db.Save(&s).Error; err != nil {
		t.Fatalf("0084 write path still failing after migration: %v", err)
	}

	var got models.Scene
	if err := db.Where("scene_id = ?", "naughtyamerica-vr-test").First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.TrailerType != "heresphere" || got.TrailerSource != `{"SceneUrl":"https://api.naughtyapi.com/heresphere/test"}` {
		t.Errorf("trailer fields = %q/%q, want migrated values", got.TrailerType, got.TrailerSource)
	}
}

// On a fresh database the tables don't exist yet; the ensure-migration
// must no-op successfully (0001's full-model AutoMigrate creates the
// tables complete right after). Migrating the stub structs blindly fails
// with "Cannot add a PRIMARY KEY column" and kills first boot.
func TestEnsureFunscriptSpeedColumnsFreshDB(t *testing.T) {
	db, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "migration_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := Migrate0000EnsureFunscriptSpeedColumns(db); err != nil {
		t.Fatalf("migration failed on fresh database: %v", err)
	}
}
