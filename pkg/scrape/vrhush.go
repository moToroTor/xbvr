package scrape

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/gocolly/colly/v2"
	"github.com/mozillazg/go-slugify"
	"github.com/nleeper/goment"
	"github.com/thoas/go-funk"
	"github.com/xbapps/xbvr/pkg/models"
)

func vrhushString(m map[string]interface{}, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return s, true
}

func vrhushMap(m map[string]interface{}, key string) (map[string]interface{}, bool) {
	if m == nil {
		return nil, false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil, false
	}
	mp, ok := v.(map[string]interface{})
	if !ok || mp == nil {
		return nil, false
	}
	return mp, true
}

func vrhushSlice(m map[string]interface{}, key string) ([]interface{}, bool) {
	if m == nil {
		return nil, false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil, false
	}
	s, ok := v.([]interface{})
	if !ok || s == nil {
		return nil, false
	}
	return s, true
}

func VRHush(wg *models.ScrapeWG, updateSite bool, knownScenes []string, out chan<- models.ScrapedScene, singleSceneURL string, singeScrapeAdditionalInfo string, limitScraping bool) error {
	defer wg.Done()
	scraperID := "vrhush"
	siteID := "VRHush"
	logScrapeStart(scraperID, siteID)

	sceneCollector := createCollector("vrhush.com")
	siteCollector := createCollector("vrhush.com")
	pageCnt := 1

	sceneCollector.OnHTML(`html`, func(e *colly.HTMLElement) {
		sc := models.ScrapedScene{}
		sc.ScraperID = scraperID
		sc.SceneType = "VR"
		sc.Studio = "VRHush"
		sc.Site = siteID
		sc.HomepageURL = strings.Split(e.Request.URL.String(), "?")[0]
		sc.MembersUrl = strings.Replace(sc.HomepageURL, "https://vrhush.com/scenes/", "https://ma.vrhush.com/scene/", 1)

		// get json data
		var jsonResult map[string]interface{}
		e.ForEach(`script[Id="__NEXT_DATA__"]`, func(id int, e *colly.HTMLElement) {
			json.Unmarshal([]byte(e.Text), &jsonResult)
		})
		props, ok := vrhushMap(jsonResult, "props")
		if !ok {
			return
		}
		pageProps, ok := vrhushMap(props, "pageProps")
		if !ok {
			return
		}
		jsonResult = pageProps
		content, ok := vrhushMap(pageProps, "content")
		if !ok {
			return
		}

		// Scene ID - get from json scene code (url no longer has the code)
		sceneCode, ok := vrhushString(content, "scene_code")
		if !ok || sceneCode == "" {
			log.Warnf("Unable to process %s - no scene code", e.Request.URL)
			return
		}
		tmp := strings.Split(sceneCode, "_")[0]
		sc.SiteID = strings.Replace(tmp, "vrh", "", -1)
		sc.SceneID = slugify.Slugify(sc.Site) + "-" + sc.SiteID

		// Title / Cover
		if title, ok := vrhushString(content, "title"); ok {
			sc.Title = title
		}

		if screencap, ok := vrhushString(content, "trailer_screencap"); ok && screencap != "" {
			sc.Covers = append(sc.Covers, e.Request.AbsoluteURL(screencap))
		}

		// Synopsis
		if synopsis, ok := vrhushString(content, "description"); ok {
			sc.Synopsis = synopsis
		}

		// Tags
		if tagList, ok := vrhushSlice(content, "tags"); ok {
			for _, tag := range tagList {
				if tagStr, ok := tag.(string); ok {
					sc.Tags = append(sc.Tags, tagStr)
				}
			}
		}

		// Cast
		sc.ActorDetails = make(map[string]models.ActorDetails)
		modelList, ok := vrhushSlice(jsonResult, "models")
		if !ok {
			modelList, ok = vrhushSlice(content, "models")
		}
		if ok {
			for _, model := range modelList {
				modelMap, ok := model.(map[string]interface{})
				if !ok || modelMap == nil {
					continue
				}
				gender, _ := vrhushString(modelMap, "gender")
				if gender == "Female" {
					name, ok := vrhushString(modelMap, "name")
					if !ok || name == "" {
						continue
					}
					sc.Cast = append(sc.Cast, name)
					slug, _ := vrhushString(modelMap, "slug")
					sc.ActorDetails[name] = models.ActorDetails{Source: sc.ScraperID + " scrape", ProfileUrl: "https://vrhush.com/models/" + slug}
				}
			}
		}

		// Date & duration
		if publishDate, ok := vrhushString(content, "publish_date"); ok && publishDate != "" {
			tmpDate, _ := goment.New(publishDate, "YYYY/MM/DD")
			sc.Released = tmpDate.Format("YYYY-MM-DD")
		}
		if v, ok := content["videos_duration"]; ok && v != nil {
			switch dur := v.(type) {
			case string:
				if dur != "" {
					num, _ := strconv.ParseFloat(dur, 64)
					sc.Duration = int(num / 60)
				}
			case float64:
				sc.Duration = int(dur / 60)
			case float32:
				sc.Duration = int(float64(dur) / 60)
			case int:
				sc.Duration = dur / 60
			case int64:
				sc.Duration = int(dur / 60)
			}
		}
		// trailer details

		sc.TrailerType = "scrape_json"
		var t models.TrailerScrape
		t.SceneUrl = sc.HomepageURL
		t.HtmlElement = `script[id="__NEXT_DATA__"]`
		t.RecordPath = "props.pageProps.content.trailers"
		t.ContentPath = "url"
		t.QualityPath = "label"
		t.ContentBaseUrl = "https:"
		tmpjson, _ := json.Marshal(t)
		sc.TrailerSrc = string(tmpjson)

		// Filenames
		if videos, ok := vrhushMap(content, "videos"); ok {
			for _, video := range videos {
				videoMap, ok := video.(map[string]interface{})
				if !ok || videoMap == nil {
					continue
				}
				if file, ok := vrhushString(videoMap, "file"); ok && file != "" {
					tmp := strings.Split(file, "/")
					sc.Filenames = append(sc.Filenames, tmp[len(tmp)-1])
				} else {
					if rawURL, ok := vrhushString(videoMap, "url"); ok && rawURL != "" {
						parsedURL, _ := url.Parse(rawURL)
						baseName := path.Base(parsedURL.Path)
						sc.Filenames = append(sc.Filenames, baseName)
					}
				}
			}
		}

		out <- sc
	})

	siteCollector.OnHTML(`ul.pagination li`, func(e *colly.HTMLElement) {
		if strings.Contains(e.Attr("class"), "next") && !strings.Contains(e.Attr("class"), "disabled") {
			pageCnt += 1
			if !limitScraping {
				pageURL := e.Request.AbsoluteURL(`https://vrhush.com/scenes?page=` + fmt.Sprint(pageCnt) + `&order_by=publish_date&sort_by=desc`)
				siteCollector.Visit(pageURL)
			}
		}
	})

	siteCollector.OnHTML(`div.contentThumb__info__title A`, func(e *colly.HTMLElement) {
		sceneURL := e.Request.AbsoluteURL(e.Attr("href"))

		// If scene exist in database, there's no need to scrape
		if !funk.ContainsString(knownScenes, sceneURL) {
			sceneCollector.Visit(sceneURL)
		}
	})

	if singleSceneURL != "" {
		sceneCollector.Visit(singleSceneURL)
	} else {
		siteCollector.Visit("https://vrhush.com/scenes?page=1&order_by=publish_date&sort_by=desc")
	}

	if updateSite {
		updateSiteLastUpdate(scraperID)
	}
	logScrapeFinished(scraperID, siteID)
	return nil
}

func init() {
	registerScraper("vrhush", "VRHush", "https://cdn-nexpectation.secure.yourpornpartner.com/sites/vrh/favicon/apple-touch-icon-180x180.png", "vrhush.com", VRHush)
}
