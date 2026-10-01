package models

import (
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/jinzhu/gorm"
)

type Site struct {
	ID             string    `gorm:"primary_key" json:"id" xbvrbackup:"-"`
	Name           string    `json:"name"  xbvrbackup:"name"`
	AvatarURL      string    `json:"avatar_url" xbvrbackup:"-"`
	IsBuiltin      bool      `json:"is_builtin" xbvrbackup:"-"`
	IsEnabled      bool      `json:"is_enabled" xbvrbackup:"is_enabled"`
	LastUpdate     time.Time `json:"last_update" xbvrbackup:"-"`
	Subscribed     bool      `json:"subscribed" xbvrbackup:"subscribed"`
	HasScraper     bool      `gorm:"-" json:"has_scraper" xbvrbackup:"-"`
	LimitScraping  bool      `json:"limit_scraping" xbvrbackup:"limit_scraping"`
	MasterSiteID   string    `json:"master_site_id" xbvrbackup:"master_site_id"`
	MatchingParams string    `json:"matching_params" gorm:"size:1000" xbvrbackup:"matching_params"`
	ScrapeStash    bool      `json:"scrape_stash" xbvrbackup:"scrape_stash"`
	SceneCount     int       `gorm:"-" json:"scene_count" xbvrbackup:"-"`
	// Last scrape run outcome, for the "why is this site empty" status.
	// Ephemeral diagnostics, hence excluded from backups.
	LastScrapeStartedAt  time.Time `json:"last_scrape_started_at" xbvrbackup:"-"`
	LastScrapeFinishedAt time.Time `json:"last_scrape_finished_at" xbvrbackup:"-"`
	LastScrapeNewScenes  int       `json:"last_scrape_new_scenes" xbvrbackup:"-"`
	LastScrapeBlocked    int       `json:"last_scrape_blocked" xbvrbackup:"-"`
	LastScrapeErrors     int       `json:"last_scrape_errors" xbvrbackup:"-"`
}

func (i *Site) Save() error {
	db, _ := GetDB()
	defer db.Close()

	var err error = retry.Do(
		func() error {
			err := db.Save(&i).Error
			if err != nil {
				return err
			}
			return nil
		},
	)

	if err != nil {
		log.Fatal("Failed to save ", err)
	}

	return nil
}

func (i *Site) GetIfExist(id string) error {
	db, _ := GetDB()
	defer db.Close()

	return db.Where(&Site{ID: id}).First(i).Error
}

func InitSites() {
	db, _ := GetDB()
	defer db.Close()

	scrapers := GetScrapers()
	for i := range scrapers {
		if !strings.HasSuffix(scrapers[i].ID, "-single_scene") {
			if err := initSiteRecord(db, scrapers[i]); err != nil {
				log.Fatal("Failed to init site ", err)
			}
		}
	}
}

// initSiteRecord upserts one builtin site row, writing only the columns
// InitSites manages. Schema migrations run in the background while
// InitSites runs at startup, so a full-struct save can reference columns a
// pending migration has not added yet (e.g. the 0091 run-status columns on
// a pre-0090 database: "no such column", retried, then log.Fatal) and take
// the process down before migrations finish.
func initSiteRecord(db *gorm.DB, scraper Scraper) error {
	attrs := map[string]interface{}{
		"name":           scraper.Name,
		"avatar_url":     scraper.AvatarURL,
		"is_builtin":     true,
		"master_site_id": scraper.MasterSiteId,
	}
	// Writes use explicit table/column references only: a full-struct
	// write would drag in columns a pending migration has not added yet.
	// (Reads are safe with the struct: SELECT * only returns columns that
	// exist, and absent fields stay zero.)
	if db.Where("id = ?", scraper.ID).First(&Site{}).RecordNotFound() {
		// Explicit column list (this gorm version drops map values on
		// Create, emitting DEFAULT VALUES instead).
		return db.Exec(
			`INSERT INTO sites (id, name, avatar_url, is_builtin, master_site_id) VALUES (?, ?, ?, ?, ?)`,
			scraper.ID, scraper.Name, scraper.AvatarURL, true, scraper.MasterSiteId,
		).Error
	}
	return db.Table("sites").Where("id = ?", scraper.ID).UpdateColumns(attrs).Error
}
