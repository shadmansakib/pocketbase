package apis

import (
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

func bindRecordActionsApi(_ core.App, rg *router.RouterGroup[*core.RequestEvent]) {
	sub := rg.Group("/collections/{collection}/records/actions")
	sub.GET("", recordActionsList).Bind(RequireSuperuserAuth())
	sub.POST("/{action}", recordActionExecute).Bind(RequireSuperuserAuth())
}

type recordActionExecuteForm struct {
	Ids     []string       `json:"ids"`
	Payload map[string]any `json:"payload"`
}

func recordActionsList(e *core.RequestEvent) error {
	collection, err := e.App.FindCachedCollectionByNameOrId(e.Request.PathValue("collection"))
	if err != nil || collection == nil {
		return e.NotFoundError("Missing collection context.", err)
	}

	actions := e.App.CollectionActions().List(collection)
	result := make([]map[string]any, 0, len(actions))
	for _, action := range actions {
		result = append(result, collectionActionResponse(action))
	}

	return e.JSON(http.StatusOK, result)
}

func recordActionExecute(e *core.RequestEvent) error {
	collection, err := e.App.FindCachedCollectionByNameOrId(e.Request.PathValue("collection"))
	if err != nil || collection == nil {
		return e.NotFoundError("Missing collection context.", err)
	}

	actionName := e.Request.PathValue("action")
	action, ok := e.App.CollectionActions().Resolve(collection, actionName)
	if !ok || action == nil {
		return e.NotFoundError("Missing or invalid collection action.", nil)
	}

	form := &recordActionExecuteForm{Payload: map[string]any{}}
	if err := e.BindBody(form); err != nil {
		return e.BadRequestError("Failed to read the submitted action data.", err)
	}

	form.Ids = core.NormalizeCollectionActionSelectedIds(form.Ids)

	err = validation.ValidateStruct(form,
		validation.Field(&form.Ids, validation.Required, validation.By(func(value any) error {
			ids, _ := value.([]string)
			if len(ids) == 0 {
				return validation.ErrRequired
			}
			return nil
		})),
	)
	if err != nil {
		return e.BadRequestError("Invalid action data.", err)
	}

	if err := core.ValidateCollectionActionSelection(action, form.Ids); err != nil {
		return e.BadRequestError(err.Error(), err)
	}

	records, err := core.LoadCollectionActionRecords(e.App, collection, action, form.Ids)
	if err != nil {
		return e.BadRequestError(err.Error(), err)
	}

	result, err := core.RunCollectionActionHandler(e.App, collection, action, form.Ids, records, form.Payload)
	if err != nil {
		return e.BadRequestError("Action failed.", err)
	}

	if action.ClearSelection {
		result["clearSelection"] = true
	}
	if action.ReloadRecords {
		result["reloadRecords"] = true
	}
	if result["message"] == nil || result["message"] == "" {
		result["message"] = "Action completed successfully."
	}

	return e.JSON(http.StatusOK, result)
}

func collectionActionResponse(action *core.CollectionAction) map[string]any {
	if action == nil {
		return nil
	}

	return map[string]any{
		"name":               action.Name,
		"label":              action.Label,
		"description":        action.Description,
		"icon":               action.Icon,
		"order":              action.Order,
		"collections":        action.Collections,
		"excludeCollections": action.ExcludeCollections,
		"selectionRequired":  action.SelectionRequired,
		"minSelection":       action.MinSelection,
		"maxSelection":       action.MaxSelection,
		"confirmText":        action.ConfirmText,
		"variant":            action.Variant,
		"loadRecords":        action.LoadRecords,
		"clearSelection":     action.ClearSelection,
		"reloadRecords":      action.ReloadRecords,
	}
}
