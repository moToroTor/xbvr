package tasks

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/xbapps/xbvr/pkg/models"
)

// Duplicate detection for scenes scraped twice under different scraper IDs
// (a second scraper added without master-site/matching config, so the
// alternate-source relink never attempts them). Groups by normalized title
// plus duration — the same normalization the relink matcher uses, so a
// group the detector reports is one the matcher would have linked had it
// been configured.

// NormalizeDupTitle strips the punctuation/whitespace the relink matcher
// ignores (see MatchAlternateSources) so detector groups and matcher
// behavior agree.
func NormalizeDupTitle(title string) string {
	tmp := strings.ReplaceAll(strings.ReplaceAll(title, " and ", "&"), "+", "&")
	for _, char := range `'- :!?"/.,@()–` {
		tmp = strings.ReplaceAll(tmp, string(char), "")
	}
	return strings.ToLower(tmp)
}

// DupScene is one member of a duplicate group, with the fields the review
// UI needs to tell candidates apart.
type DupScene struct {
	ID         uint   `json:"id"`
	SceneID    string `json:"scene_id"`
	ScraperID  string `json:"scraper_id"`
	Site       string `json:"site"`
	Title      string `json:"title"`
	Duration   int    `json:"duration"`
	CoverURL   string `json:"cover_url"`
	FileCount  int    `json:"file_count"`
	HasFiles   bool   `json:"has_files"`
	ReleaseDay string `json:"release_day"`
}

// DuplicateGroup is a set of live scenes, each from a different scraper,
// that normalize to the same title+duration.
type DuplicateGroup struct {
	Key           string     `json:"key"`
	Scenes        []DupScene `json:"scenes"`
	SuggestedKeep uint       `json:"suggested_keep"`
	SuggestWhy    string     `json:"suggest_why"`
}

type dupRow struct {
	ID          uint
	SceneID     string
	ScraperID   string
	Site        string
	Title       string
	Duration    int
	CoverURL    string
	ReleaseDate time.Time
}

// groupDupRows clusters rows by normalized title+duration, keeping only
// groups spanning more than one scraper. Pure for testability.
func groupDupRows(rows []dupRow) [][]dupRow {
	byKey := map[string][]dupRow{}
	for _, r := range rows {
		nt := NormalizeDupTitle(r.Title)
		if nt == "" || r.Duration <= 0 {
			continue
		}
		byKey[nt+"\x00"+strconv.Itoa(r.Duration)] = append(byKey[nt+"\x00"+strconv.Itoa(r.Duration)], r)
	}
	var out [][]dupRow
	for _, g := range byKey {
		scrapers := map[string]bool{}
		for _, r := range g {
			scrapers[r.ScraperID] = true
		}
		if len(scrapers) < 2 {
			continue
		}
		sort.Slice(g, func(a, b int) bool { return g[a].ScraperID < g[b].ScraperID })
		out = append(out, g)
	}
	sort.Slice(out, func(a, b int) bool { return out[a][0].ScraperID < out[b][0].ScraperID })
	return out
}

const dupesDismissedKey = "dupes_dismissed"

func dupPairKey(a, b uint) string {
	if a > b {
		a, b = b, a
	}
	return strconv.FormatUint(uint64(a), 10) + ":" + strconv.FormatUint(uint64(b), 10)
}

func loadDismissed(db *gorm.DB) map[string]bool {
	var kv models.KV
	dismissed := map[string]bool{}
	if err := db.Where(&models.KV{Key: dupesDismissedKey}).First(&kv).Error; err != nil {
		return dismissed
	}
	var keys []string
	if err := json.Unmarshal([]byte(kv.Value), &keys); err != nil {
		return dismissed
	}
	for _, k := range keys {
		dismissed[k] = true
	}
	return dismissed
}

// scenesLinked reports whether two scenes are already connected through an
// alternate-source link pairing one scene's row with the other's scene_id,
// in either direction.
func scenesLinked(db *gorm.DB, aID uint, aSceneID string, bID uint, bSceneID string) bool {
	var n int
	db.Table("external_reference_links").
		Where("internal_table = ? AND ((internal_db_id = ? AND external_id = ?) OR (internal_db_id = ? AND external_id = ?))",
			"scenes", aID, bSceneID, bID, aSceneID).Count(&n)
	return n > 0
}

func fileCount(db *gorm.DB, sceneID uint) int {
	var n int
	db.Table("files").Where("scene_id = ?", sceneID).Count(&n)
	return n
}

// FindDuplicateGroups returns live-scene groups the relink matcher would
// link if it were configured: same normalized title+duration across
// scrapers, not already linked, not dismissed.
func FindDuplicateGroups(db *gorm.DB) ([]DuplicateGroup, error) {
	var rows []dupRow
	if err := db.Table("scenes").
		Select("id, scene_id, scraper_id, site, title, duration, cover_url, release_date").
		Where("deleted_at IS NULL").Find(&rows).Error; err != nil {
		return nil, err
	}
	dismissed := loadDismissed(db)
	var out []DuplicateGroup
	for _, g := range groupDupRows(rows) {
		var members []DupScene
		for _, r := range g {
			day := ""
			if !r.ReleaseDate.IsZero() {
				day = r.ReleaseDate.Format("2006-01-02")
			}
			fc := fileCount(db, r.ID)
			members = append(members, DupScene{
				ID: r.ID, SceneID: r.SceneID, ScraperID: r.ScraperID,
				Site: r.Site, Title: r.Title, Duration: r.Duration,
				CoverURL: r.CoverURL, FileCount: fc, HasFiles: fc > 0,
				ReleaseDay: day,
			})
		}
		// Drop members already linked as alternates; a group that no
		// longer spans scrapers is done.
		var unlinked []DupScene
		for _, s := range members {
			linked := false
			for _, o := range members {
				if o.ID != s.ID && scenesLinked(db, s.ID, s.SceneID, o.ID, o.SceneID) {
					linked = true
					break
				}
			}
			if !linked {
				unlinked = append(unlinked, s)
			}
		}
		scrapers := map[string]bool{}
		for _, s := range unlinked {
			scrapers[s.ScraperID] = true
		}
		if len(scrapers) < 2 {
			continue
		}
		allDismissed := true
		for i := 0; i < len(unlinked); i++ {
			for j := i + 1; j < len(unlinked); j++ {
				if !dismissed[dupPairKey(unlinked[i].ID, unlinked[j].ID)] {
					allDismissed = false
				}
			}
		}
		if allDismissed {
			continue
		}
		keep, why := suggestKeep(unlinked)
		out = append(out, DuplicateGroup{
			Key:           dupPairKey(unlinked[0].ID, unlinked[len(unlinked)-1].ID),
			Scenes:        unlinked,
			SuggestedKeep: keep,
			SuggestWhy:    why,
		})
	}
	return out, nil
}

// suggestKeep prefers the scene with files matched (linking must not orphan
// file matches), else the oldest row.
func suggestKeep(members []DupScene) (uint, string) {
	var withFiles []DupScene
	for _, s := range members {
		if s.HasFiles {
			withFiles = append(withFiles, s)
		}
	}
	pool, why := members, "oldest record"
	if len(withFiles) > 0 {
		pool, why = withFiles, "has matched files"
	}
	best := pool[0]
	for _, s := range pool[1:] {
		if s.ID < best.ID {
			best = s
		}
	}
	return best.ID, why
}

// DismissDuplicate records a pair so the detector stops surfacing it.
func DismissDuplicate(db *gorm.DB, aID, bID uint) error {
	dismissed := loadDismissed(db)
	dismissed[dupPairKey(aID, bID)] = true
	var keys []string
	for k := range dismissed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	raw, err := json.Marshal(keys)
	if err != nil {
		return err
	}
	return db.Save(&models.KV{Key: dupesDismissedKey, Value: string(raw)}).Error
}

// LinkDuplicate links loser as an alternate source under winner, exactly as
// the relink task records a confirmed match (manual match_type 99999, which
// reprocessing leaves alone). Nothing is deleted; files stay where they are.
func LinkDuplicate(db *gorm.DB, winnerID, loserID uint) error {
	var winner models.Scene
	if err := db.Where("id = ?", winnerID).First(&winner).Error; err != nil {
		return err
	}
	var loser models.Scene
	if err := db.Preload("Cast").Where("id = ?", loserID).First(&loser).Error; err != nil {
		return err
	}

	// Strip actor columns as the relink task does; only names are needed.
	var newCastList []models.Actor
	for _, actor := range loser.Cast {
		newCastList = append(newCastList, models.Actor{ID: actor.ID, Name: actor.Name})
	}
	loser.Cast = newCastList

	source := "alternate scene " + loser.ScraperId
	var extref models.ExternalReference
	if err := db.Where(&models.ExternalReference{ExternalSource: source, ExternalId: loser.SceneID}).First(&extref).Error; err != nil {
		extref = models.ExternalReference{ExternalSource: source, ExternalId: loser.SceneID}
	}
	extref.ExternalURL = loser.SceneURL
	data := models.SceneAlternateSource{
		MasterSiteId:  winner.ScraperId,
		Scene:         loser,
		MatchAchieved: 99999,
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	extref.ExternalData = string(raw)
	if err := db.Save(&extref).Error; err != nil {
		return err
	}
	// Same replace semantics as UpdateLinks (one link per source), but
	// against the passed handle: the shared helper targets the global DB.
	var existing []models.ExternalReferenceLink
	db.Where("external_reference_id = ?", extref.ID).Find(&existing)
	linked := false
	for _, l := range existing {
		if l.InternalDbId == winner.ID {
			linked = true
		} else {
			db.Delete(&l)
		}
	}
	if !linked {
		if err := db.Create(&models.ExternalReferenceLink{
			InternalTable: "scenes", InternalDbId: winner.ID, InternalNameId: winner.SceneID,
			ExternalReferenceID: extref.ID, ExternalSource: source, ExternalId: loser.SceneID,
			MatchType: 99999, UdfDatetime1: time.Now(),
		}).Error; err != nil {
			return err
		}
	}
	return nil
}
