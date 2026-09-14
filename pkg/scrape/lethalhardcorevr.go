package scrape

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/gocolly/colly/v2"
	"github.com/mozillazg/go-slugify"
	"github.com/nleeper/goment"
	"github.com/thoas/go-funk"
	"github.com/tidwall/gjson"
	"github.com/xbapps/xbvr/pkg/models"
)

func isGoodTag(lookup string) bool {
	switch lookup {
	case
		"vr",
		"whorecraft",
		"video",
		"streaming",
		"porn",
		"movie":
		return false
	}
	return true
}

func lethalHardcoreScene(jsonString string, queryStr string, scraperID string, siteID string, siteHost string) models.ScrapedScene {
	sc := models.ScrapedScene{}
	sc.ScraperID = scraperID
	sc.SceneType = "VR"
	sc.Studio = "Celestial Productions"
	sc.Site = siteID
	sc.SiteID = gjson.Get(jsonString, queryStr+`.clip_id`).String()
	sc.SceneID = slugify.Slugify(sc.Site) + "-" + sc.SiteID
	sc.HomepageURL = "https://www." + siteHost + "/en/video/" + scraperID + "/" +
		gjson.Get(jsonString, queryStr+`.url_title`).String() + "/" + sc.SiteID

	sc.Title = strings.TrimSpace(gjson.Get(jsonString, queryStr+`.title`).String())

	if d, err := goment.New(gjson.Get(jsonString, queryStr+`.release_date`).String(), "YYYY-MM-DD"); err == nil {
		sc.Released = d.Format("YYYY-MM-DD")
	}

	sc.Duration = int(gjson.Get(jsonString, queryStr+`.length`).Int()) / 60

	synopsis := gjson.Get(jsonString, queryStr+`.description`).String()
	if strings.TrimSpace(synopsis) == "" {
		synopsis = gjson.Get(jsonString, queryStr+`.movie_desc`).String()
	}
	sc.Synopsis = strings.TrimSpace(strings.ReplaceAll(synopsis, "</br></br>", " "))

	if cover := gjson.Get(jsonString, queryStr+`.pictures.1920x1080`).String(); cover != "" {
		sc.Covers = append(sc.Covers, "https://transform.gammacdn.com/movies/"+cover)
	}

	sc.ActorDetails = make(map[string]models.ActorDetails)
	for i := range gjson.Get(jsonString, queryStr+`.actors`).Array() {
		actorQuery := queryStr + `.actors.` + strconv.Itoa(i)
		name := strings.TrimSpace(gjson.Get(jsonString, actorQuery+`.name`).String())
		if name == "" || funk.ContainsString(sc.Cast, name) {
			continue
		}
		sc.Cast = append(sc.Cast, name)
		sc.ActorDetails[name] = models.ActorDetails{
			Source: scraperID + " scrape",
			ProfileUrl: "https://www." + siteHost + "/en/pornstar/view/" +
				gjson.Get(jsonString, actorQuery+`.url_name`).String() + "/" +
				gjson.Get(jsonString, actorQuery+`.actor_id`).String(),
		}
	}

	for _, name := range gjson.Get(jsonString, queryStr+`.categories.#.name`).Array() {
		tag := strings.ToLower(strings.TrimSpace(name.String()))
		if isGoodTag(tag) && !funk.ContainsString(sc.Tags, tag) {
			sc.Tags = append(sc.Tags, tag)
		}
	}

	if trailer := gjson.Get(jsonString, queryStr+`.trailers.0.url`).String(); trailer != "" {
		sc.TrailerType = "url"
		sc.TrailerSrc = trailer
	}

	return sc
}

func LethalHardcoreSite(wg *models.ScrapeWG, updateSite bool, knownScenes []string, out chan<- models.ScrapedScene, singleSceneURL string, scraperID string, siteID string, siteHost string, singeScrapeAdditionalInfo string, limitScraping bool) error {
	defer wg.Done()
	logScrapeStart(scraperID, siteID)

	// The site is a React app that renders nothing server side; its data comes
	// from the shared Gamma Algolia index, keyed by availableOnSite.
	keyCollector := createCollector(siteHost)

	var apiKey, appID string
	keyCollector.OnHTML(`html`, func(e *colly.HTMLElement) {
		body, err := e.DOM.Html()
		if err != nil {
			return
		}
		if m := regexp.MustCompile(`"apiKey":"([^"]+)"`).FindStringSubmatch(body); m != nil {
			apiKey = m[1]
		}
		if m := regexp.MustCompile(`"applicationID":"([^"]+)"`).FindStringSubmatch(body); m != nil {
			appID = m[1]
		}
	})
	keyCollector.Visit("https://www." + siteHost + "/en/videos")

	if apiKey == "" || appID == "" {
		log.Errorf("%s: could not read Algolia credentials from the site", siteID)
		logScrapeFinished(scraperID, siteID)
		return nil
	}

	query := func(params string) (string, error) {
		payload := `{"requests":[{"indexName":"all_scenes","params":"` + params + `"}]}`
		r, err := resty.New().R().
			SetHeader("Origin", "https://www."+siteHost).
			SetHeader("Referer", "https://www."+siteHost+"/").
			SetHeader("Content-Type", "application/json").
			SetHeader("x-algolia-api-key", apiKey).
			SetHeader("x-algolia-application-id", appID).
			SetBody(payload).
			Post("https://" + strings.ToLower(appID) + "-dsn.algolia.net/1/indexes/*/queries")
		if err != nil {
			return "", err
		}
		return r.String(), nil
	}

	if singleSceneURL != "" {
		parts := strings.Split(strings.TrimSuffix(singleSceneURL, "/"), "/")
		sceneID := parts[len(parts)-1]
		jsonString, err := query("facetFilters=%5B%5B%22availableOnSite%3A" + scraperID +
			"%22%5D%2C%5B%22clip_id%3A" + sceneID + "%22%5D%5D&hitsPerPage=1")
		if err != nil {
			log.Errorln(siteID, err)
		} else if len(gjson.Get(jsonString, "results.0.hits").Array()) > 0 {
			out <- lethalHardcoreScene(jsonString, "results.0.hits.0", scraperID, siteID, siteHost)
		}
	} else {
		page := 0
		for {
			jsonString, err := query("facetFilters=%5B%5B%22availableOnSite%3A" + scraperID +
				"%22%5D%5D&hitsPerPage=60&page=" + strconv.Itoa(page))
			if err != nil {
				log.Errorln(siteID, err)
				break
			}
			hits := gjson.Get(jsonString, "results.0.hits").Array()
			if len(hits) == 0 {
				break
			}
			for i := range hits {
				sc := lethalHardcoreScene(jsonString, "results.0.hits."+strconv.Itoa(i), scraperID, siteID, siteHost)
				if sc.SceneID != "" && !isKnownScene(scraperID, knownScenes, sc.HomepageURL) {
					out <- sc
				}
			}
			page++
			if limitScraping || page >= int(gjson.Get(jsonString, "results.0.nbPages").Int()) {
				break
			}
		}
	}

	if updateSite {
		updateSiteLastUpdate(scraperID)
	}
	logScrapeFinished(scraperID, siteID)
	return nil
}

func LethalHardcoreVR(wg *models.ScrapeWG, updateSite bool, knownScenes []string, out chan<- models.ScrapedScene, singleSceneURL string, singeScrapeAdditionalInfo string, limitScraping bool) error {
	return LethalHardcoreSite(wg, updateSite, knownScenes, out, singleSceneURL, "lethalhardcorevr", "LethalHardcoreVR", "lethalhardcorevr.com", singeScrapeAdditionalInfo, limitScraping)
}

func WhorecraftVR(wg *models.ScrapeWG, updateSite bool, knownScenes []string, out chan<- models.ScrapedScene, singleSceneURL string, singeScrapeAdditionalInfo string, limitScraping bool) error {
	return LethalHardcoreSite(wg, updateSite, knownScenes, out, singleSceneURL, "whorecraftvr", "WhorecraftVR", "lethalhardcorevr.com", singeScrapeAdditionalInfo, limitScraping)
}

func init() {
	registerScraper("whorecraftvr", "WhorecraftVR", "https://imgs1cdn.adultempire.com/bn/Whorecraft-VR-apple-touch-icon.png", "lethalhardcorevr.com", WhorecraftVR)
	registerScraper("lethalhardcorevr", "LethalHardcoreVR", "https://imgs1cdn.adultempire.com/bn/Lethal-Hardcore-apple-touch-icon.png", "lethalhardcorevr.com", LethalHardcoreVR)
}
