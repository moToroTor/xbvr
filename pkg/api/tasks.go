package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/xbapps/xbvr/pkg/models"
	"github.com/xbapps/xbvr/pkg/scrape"
	"github.com/xbapps/xbvr/pkg/tasks"
)

type RequestScrapeJAVR struct {
	Scraper string `json:"s"`
	Query   string `json:"q"`
}

type RequestScrapeJAVRBatch struct {
	Scraper string `json:"s"`
	Codes   string `json:"codes"`
}

type RequestScrapeTPDB struct {
	ApiToken string `json:"apiToken"`
	SceneUrl string `json:"sceneUrl"`
}

type RequestScrapeSearch struct {
	Query      string   `json:"q"`
	Site       string   `json:"site"`
	Performers []string `json:"performers"`
}

type RequestScrapePick struct {
	Title      string   `json:"title"`
	Site       string   `json:"site"`
	Performers []string `json:"performers"`
	ScraperID  string   `json:"scraper_id"`
	URL        string   `json:"url"`
	Wishlist   bool     `json:"wishlist"`
}

type ResponseScrapePick struct {
	Response   string                  `json:"status"`
	SceneID    uint                    `json:"scene_id"`
	Wishlisted bool                    `json:"wishlisted"`
	Candidate  *scrape.ScrapeCandidate `json:"candidate"`
}

type ResponseScrapeSearch struct {
	Response     string                   `json:"status"`
	Candidates   []scrape.ScrapeCandidate `json:"candidates"`
	UnknownHosts []string                 `json:"unknown_hosts"`
}

type RequestSingleScrape struct {
	Site           string                            `json:"site"`
	SceneUrl       string                            `json:"sceneurl"`
	AdditionalInfo []RequestSingleScrapeAdditionInfo `json:"additionalinfo"`
}

type RequestSingleScrapeAdditionInfo struct {
	FieldName   string `json:"fieldName"`
	FieldPrompt string `json:"fieldPrompt"`
	Placeholder string `json:"placeholder"`
	FieldValue  string `json:"fieldValue"`
	Required    bool   `json:"required"`
	Type        string `json:"type"`
}
type ResponseBackupBundle struct {
	Response string `json:"status"`
}

type ResponseSceneScrape struct {
	Response string       `json:"status"`
	Scene    models.Scene `json:"scene"`
}

type TaskResource struct{}

func (i TaskResource) WebService() *restful.WebService {
	tags := []string{"Task"}

	ws := new(restful.WebService)

	ws.Path("/api/task").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	ws.Route(ws.GET("/rescan").To(i.rescan).
		Metadata(restfulspec.KeyOpenAPITags, tags))

	ws.Route(ws.GET("/rescan/{storage-id}").To(i.rescan).
		Metadata(restfulspec.KeyOpenAPITags, tags))

	ws.Route(ws.GET("/scene-refresh").To(i.sceneRrefresh).
		Metadata(restfulspec.KeyOpenAPITags, tags))

	ws.Route(ws.GET("/clean-tags").To(i.cleanTags).
		Metadata(restfulspec.KeyOpenAPITags, tags))

	ws.Route(ws.GET("/scrape").To(i.scrape).
		Metadata(restfulspec.KeyOpenAPITags, tags))

	ws.Route(ws.POST("/singlescrape").To(i.singleScrape).
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Writes(ResponseSceneScrape{}))

	ws.Route(ws.GET("/index").To(i.index).
		Metadata(restfulspec.KeyOpenAPITags, tags))

	ws.Route(ws.GET("/preview/generate").To(i.previewGenerate).
		Metadata(restfulspec.KeyOpenAPITags, tags))

	ws.Route(ws.GET("/funscript/export-all").To(i.exportAllFunscripts).
		Metadata(restfulspec.KeyOpenAPITags, tags))

	ws.Route(ws.GET("/funscript/export-new").To(i.exportNewFunscripts).
		Metadata(restfulspec.KeyOpenAPITags, tags))

	ws.Route(ws.GET("/bundle/backup").To(i.backupBundle).
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Writes(ResponseBackupBundle{}))

	ws.Route(ws.POST("/bundle/restore").To(i.restoreBundle).
		Metadata(restfulspec.KeyOpenAPITags, tags))

	ws.Route(ws.POST("/scrape-javr").To(i.scrapeJAVR).
		Metadata(restfulspec.KeyOpenAPITags, tags))

	ws.Route(ws.POST("/scrape-javr-batch").To(i.scrapeJAVRBatch).
		Metadata(restfulspec.KeyOpenAPITags, tags))

	ws.Route(ws.POST("/scrape-tpdb").To(i.scrapeTPDB).
		Metadata(restfulspec.KeyOpenAPITags, tags))

	ws.Route(ws.POST("/scrape-search").To(i.scrapeSearch).
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Writes(ResponseScrapeSearch{}))

	ws.Route(ws.POST("/scrape-pick").To(i.scrapePick).
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Writes(ResponseScrapePick{}))

	ws.Route(ws.GET("/relink_alt_aource_scenes").To(i.relink_alt_aource_scenes).
		Metadata(restfulspec.KeyOpenAPITags, tags))
	return ws
}

func (i TaskResource) rescan(req *restful.Request, resp *restful.Response) {
	id, err := strconv.Atoi(req.PathParameter("storage-id"))
	if err != nil {
		// no storage-id, refresh all
		go tasks.RescanVolumes(-1)
		return
	} else {
		// just refresh the specified path
		go tasks.RescanVolumes(id)
	}
}

func (i TaskResource) sceneRrefresh(req *restful.Request, resp *restful.Response) {
	go tasks.RefreshSceneStatuses()
}

func (i TaskResource) cleanTags(req *restful.Request, resp *restful.Response) {
	go tasks.CleanTags()
}

func (i TaskResource) index(req *restful.Request, resp *restful.Response) {
	go tasks.SearchIndex()
}

func (i TaskResource) scrape(req *restful.Request, resp *restful.Response) {
	qSiteID := req.QueryParameter("site")
	if qSiteID == "" {
		qSiteID = "_enabled"
	}
	go tasks.Scrape(qSiteID, "", "")
}
func (i TaskResource) singleScrape(req *restful.Request, resp *restful.Response) {
	var scrapeParams RequestSingleScrape
	req.ReadEntity(&scrapeParams)
	additionalInfo, _ := json.Marshal(scrapeParams.AdditionalInfo)

	newScene := tasks.ScrapeSingleScene(scrapeParams.Site, scrapeParams.SceneUrl, string(additionalInfo))

	createResp := &ResponseSceneScrape{
		Response: "OK",
		Scene:    newScene,
	}
	resp.WriteHeaderAndEntity(http.StatusOK, createResp)
}

func (i TaskResource) exportAllFunscripts(req *restful.Request, resp *restful.Response) {
	tasks.ExportFunscripts(resp.ResponseWriter, false)
}

func (i TaskResource) exportNewFunscripts(req *restful.Request, resp *restful.Response) {
	tasks.ExportFunscripts(resp.ResponseWriter, true)
}

func (i TaskResource) backupBundle(req *restful.Request, resp *restful.Response) {
	inclAllSites, _ := strconv.ParseBool(req.QueryParameter("allSites"))
	onlyIncludeOfficalSites, _ := strconv.ParseBool(req.QueryParameter("onlyIncludeOfficalSites"))
	inclScenes, _ := strconv.ParseBool(req.QueryParameter("inclScenes"))
	inclFileLinks, _ := strconv.ParseBool(req.QueryParameter("inclLinks"))
	inclCuepoints, _ := strconv.ParseBool(req.QueryParameter("inclCuepoints"))
	inclHistory, _ := strconv.ParseBool(req.QueryParameter("inclHistory"))
	inclPlaylists, _ := strconv.ParseBool(req.QueryParameter("inclPlaylists"))
	inclActorAkas, _ := strconv.ParseBool(req.QueryParameter("inclActorAkas"))
	inclTagGroups, _ := strconv.ParseBool(req.QueryParameter("inclTagGroups"))
	inclVolumes, _ := strconv.ParseBool(req.QueryParameter("inclVolumes"))
	inclSites, _ := strconv.ParseBool(req.QueryParameter("inclSites"))
	inclActions, _ := strconv.ParseBool(req.QueryParameter("inclActions"))
	inclExtRefs, _ := strconv.ParseBool(req.QueryParameter("inclExtRefs"))
	inclActors, _ := strconv.ParseBool(req.QueryParameter("inclActors"))
	inclActorActions, _ := strconv.ParseBool(req.QueryParameter("inclActorActions"))
	inclConfig, _ := strconv.ParseBool(req.QueryParameter("inclConfig"))
	extRefSubset := req.QueryParameter("extRefSubset")
	playlistId := req.QueryParameter("playlistId")
	download := req.QueryParameter("download")

	bundle := tasks.BackupBundle(inclAllSites, onlyIncludeOfficalSites, inclScenes, inclFileLinks, inclCuepoints, inclHistory, inclPlaylists,
		inclActorAkas, inclTagGroups, inclVolumes, inclSites, inclActions, inclExtRefs, inclActors, inclActorActions, inclConfig, extRefSubset, playlistId, "", "")
	if download == "true" {
		resp.WriteHeaderAndEntity(http.StatusOK, ResponseBackupBundle{Response: "Ready to Download from http://xxx.xxx.xxx.xxx:9999/download/xbvr-content-bundle.json"})
	} else {
		// not downloading, display the bundle data
		resp.WriteHeaderAndEntity(http.StatusOK, (bundle))
	}

}

func (i TaskResource) restoreBundle(req *restful.Request, resp *restful.Response) {
	var r tasks.RequestRestore

	if err := req.ReadEntity(&r); err != nil {
		APIError(req, resp, http.StatusInternalServerError, err)
		return
	}

	go tasks.RestoreBundle(r)
}

func (i TaskResource) previewGenerate(req *restful.Request, resp *restful.Response) {
	go tasks.GeneratePreviews(nil)
}

func (i TaskResource) scrapeJAVR(req *restful.Request, resp *restful.Response) {
	var r RequestScrapeJAVR
	err := req.ReadEntity(&r)
	if err != nil {
		log.Error(err)
		return
	}

	if r.Query != "" {
		go tasks.ScrapeJAVR(r.Query, r.Scraper)
	}
}

func (i TaskResource) scrapeJAVRBatch(req *restful.Request, resp *restful.Response) {
	var r RequestScrapeJAVRBatch
	err := req.ReadEntity(&r)
	if err != nil {
		log.Error(err)
		return
	}

	codes := tasks.ParseJavCodes(r.Codes)
	if len(codes) == 0 {
		return
	}

	go tasks.ScrapeJAVRBatch(codes, r.Scraper)
	resp.WriteHeaderAndEntity(http.StatusOK, map[string]interface{}{
		"response": "OK",
		"queued":   len(codes),
	})
}

func (i TaskResource) scrapeTPDB(req *restful.Request, resp *restful.Response) {
	var r RequestScrapeTPDB
	err := req.ReadEntity(&r)
	if err != nil {
		log.Error(err)
		return
	}

	if r.ApiToken != "" && r.SceneUrl != "" {
		go tasks.ScrapeTPDB(strings.TrimSpace(r.ApiToken), strings.TrimSpace(r.SceneUrl))
	}
}
func (i TaskResource) scrapeSearch(req *restful.Request, resp *restful.Response) {
	var r RequestScrapeSearch
	if err := req.ReadEntity(&r); err != nil {
		log.Error(err)
		return
	}
	q := strings.TrimSpace(r.Query)
	if q == "" {
		resp.WriteHeaderAndEntity(http.StatusOK, ResponseScrapeSearch{Response: "OK"})
		return
	}
	out := ResponseScrapeSearch{Response: "OK", Candidates: []scrape.ScrapeCandidate{}, UnknownHosts: []string{}}
	if strings.TrimSpace(r.Site) != "" {
		cands, unknown, err := scrape.SearchScrapeCandidatesForSite(r.Site, q)
		if err != nil {
			log.Error(err)
		} else {
			out.Candidates = cands
			out.UnknownHosts = unknown
		}
	} else {
		candidates, err := scrape.SearchScrapeCandidates(q)
		if err != nil {
			log.Error(err)
		} else if candidates != nil {
			out.Candidates = candidates
		}
	}
	if out.UnknownHosts == nil {
		out.UnknownHosts = []string{}
	}
	resp.WriteHeaderAndEntity(http.StatusOK, out)
}

// scrapePick single-scrapes one web-search candidate and optionally
// wishlists it, for sidecar clients (e.g. Wankarr RSS items not yet in the
// library). With scraper_id+url the caller picked from /scrape-search
// samples; without them the server trusts its own ranking (candidates[0]).
// Idempotent: re-scraping a known URL returns the existing scene, and the
// wishlist set is a no-op the second time. SceneID 0 means nothing was
// found or the page scrape yielded no scene.
func (i TaskResource) scrapePick(req *restful.Request, resp *restful.Response) {
	var r RequestScrapePick
	if err := req.ReadEntity(&r); err != nil {
		log.Error(err)
		return
	}
	out := ResponseScrapePick{Response: "OK"}

	var cand *scrape.ScrapeCandidate
	switch {
	case r.ScraperID != "" && r.URL != "":
		cand = &scrape.ScrapeCandidate{ScraperID: r.ScraperID, URL: strings.TrimSpace(r.URL)}
	case r.ScraperID != "" || r.URL != "":
		resp.WriteErrorString(http.StatusBadRequest, "scraper_id and url are required together")
		return
	default:
		q := scrape.ComposeSearchQuery(r.Title, r.Site, r.Performers)
		if q == "" {
			resp.WriteErrorString(http.StatusBadRequest, "no query: title, site, or performers required")
			return
		}
		var candidates []scrape.ScrapeCandidate
		if strings.TrimSpace(r.Site) != "" {
			cands, _, err := scrape.SearchScrapeCandidatesForSite(r.Site, q)
			if err != nil {
				log.Error(err)
			} else {
				candidates = cands
			}
		} else {
			cands, err := scrape.SearchScrapeCandidates(q)
			if err != nil {
				log.Error(err)
			} else {
				candidates = cands
			}
		}
		if len(candidates) > 0 {
			c := candidates[0]
			cand = &c
		}
	}
	out.Candidate = cand
	if cand == nil {
		resp.WriteHeaderAndEntity(http.StatusOK, out)
		return
	}

	// Same empty-info encoding as the Files-page single scrape.
	additionalInfo, _ := json.Marshal(nil)
	scene := tasks.ScrapeSingleScene(cand.ScraperID, cand.URL, string(additionalInfo))
	if scene.ID == 0 {
		resp.WriteHeaderAndEntity(http.StatusOK, out)
		return
	}
	out.SceneID = scene.ID

	// A SET, not the /api/scene/toggle toggle: safe to repeat, and it keeps
	// the wishlist gate for scenes that already have files.
	wishlisted := scene.Wishlist
	if r.Wishlist && !wishlisted && !scene.IsAvailable {
		db, _ := models.GetDB()
		defer db.Close()
		var existing models.Scene
		if err := existing.GetIfExistByPK(scene.ID); err == nil {
			existing.Wishlist = true
			existing.Save()
			wishlisted = true
		}
	}
	out.Wishlisted = wishlisted
	resp.WriteHeaderAndEntity(http.StatusOK, out)
}

func (i TaskResource) relink_alt_aource_scenes(req *restful.Request, resp *restful.Response) {
	go tasks.MatchAlternateSources()
}
