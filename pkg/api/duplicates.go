package api

import (
	"net/http"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"

	"github.com/xbapps/xbvr/pkg/models"
	"github.com/xbapps/xbvr/pkg/tasks"
)

type DuplicatesResource struct{}

func (i DuplicatesResource) WebService() *restful.WebService {
	tags := []string{"Duplicates"}
	ws := new(restful.WebService)
	ws.Path("/api/duplicates").Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)

	ws.Route(ws.GET("").To(i.list).Metadata(restfulspec.KeyOpenAPITags, tags))
	ws.Route(ws.POST("/link").To(i.link).Metadata(restfulspec.KeyOpenAPITags, tags))
	ws.Route(ws.POST("/dismiss").To(i.dismiss).Metadata(restfulspec.KeyOpenAPITags, tags))
	return ws
}

// list returns duplicate groups for human review. Nothing is changed.
func (i DuplicatesResource) list(req *restful.Request, resp *restful.Response) {
	db, _ := models.GetDB()
	defer db.Close()
	groups, err := tasks.FindDuplicateGroups(db)
	if err != nil {
		resp.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}
	if groups == nil {
		groups = []tasks.DuplicateGroup{}
	}
	resp.WriteHeaderAndEntity(http.StatusOK, map[string]interface{}{"count": len(groups), "groups": groups})
}

// link records loser as an alternate source under winner (manual match,
// never reprocessed). Both scenes and their files are untouched.
func (i DuplicatesResource) link(req *restful.Request, resp *restful.Response) {
	var r struct {
		WinnerID uint `json:"winner_id"`
		LoserID  uint `json:"loser_id"`
	}
	if err := req.ReadEntity(&r); err != nil || r.WinnerID == 0 || r.LoserID == 0 || r.WinnerID == r.LoserID {
		resp.WriteErrorString(http.StatusBadRequest, "winner_id and loser_id (distinct) required")
		return
	}
	db, _ := models.GetDB()
	defer db.Close()
	if err := tasks.LinkDuplicate(db, r.WinnerID, r.LoserID); err != nil {
		resp.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}
	resp.WriteHeaderAndEntity(http.StatusOK, map[string]string{"status": "OK"})
}

// dismiss hides a pair from future reports.
func (i DuplicatesResource) dismiss(req *restful.Request, resp *restful.Response) {
	var r struct {
		SceneA uint `json:"scene_a"`
		SceneB uint `json:"scene_b"`
	}
	if err := req.ReadEntity(&r); err != nil || r.SceneA == 0 || r.SceneB == 0 || r.SceneA == r.SceneB {
		resp.WriteErrorString(http.StatusBadRequest, "scene_a and scene_b (distinct) required")
		return
	}
	db, _ := models.GetDB()
	defer db.Close()
	if err := tasks.DismissDuplicate(db, r.SceneA, r.SceneB); err != nil {
		resp.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}
	resp.WriteHeaderAndEntity(http.StatusOK, map[string]string{"status": "OK"})
}
