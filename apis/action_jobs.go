package apis

import (
	"net/http"
	"strconv"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

func bindActionJobsApi(_ core.App, rg *router.RouterGroup[*core.RequestEvent]) {
	sub := rg.Group("/action-jobs")
	sub.GET("", actionJobsList).Bind(RequireSuperuserAuth())
	sub.GET("/{id}", actionJobView).Bind(RequireSuperuserAuth())
}

func actionJobsList(e *core.RequestEvent) error {
	query := core.CollectionActionJobsQuery(e.App)

	if collectionRef := e.Request.URL.Query().Get("collection"); collectionRef != "" {
		collection, err := e.App.FindCachedCollectionByNameOrId(collectionRef)
		if err != nil || collection == nil {
			return e.NotFoundError("Missing collection context.", err)
		}

		query.AndWhere(dbx.Or(
			dbx.HashExp{"collectionId": collection.Id},
			dbx.HashExp{"collectionName": collection.Name},
		))
	}

	if status := e.Request.URL.Query().Get("status"); status != "" {
		query.AndWhere(dbx.HashExp{"status": status})
	}

	limit := 20
	if rawLimit := e.Request.URL.Query().Get("limit"); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 {
			if parsed > 100 {
				parsed = 100
			}
			limit = parsed
		}
	}

	actions := []*core.CollectionActionJob{}
	if err := query.OrderBy("created DESC").Limit(int64(limit)).All(&actions); err != nil {
		return e.InternalServerError("Failed to load action jobs.", err)
	}

	result := make([]map[string]any, 0, len(actions))
	for _, job := range actions {
		result = append(result, collectionActionJobResponse(job))
	}

	return e.JSON(http.StatusOK, result)
}

func actionJobView(e *core.RequestEvent) error {
	job, err := core.FindCollectionActionJobById(e.App, e.Request.PathValue("id"))
	if err != nil || job == nil {
		return e.NotFoundError("Missing action job.", err)
	}

	return e.JSON(http.StatusOK, collectionActionJobResponse(job))
}
