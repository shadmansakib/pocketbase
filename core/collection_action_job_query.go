package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/routine"
	"github.com/pocketbase/pocketbase/tools/types"
	"github.com/spf13/cast"
)

// CollectionActionJobsQuery returns a new CollectionActionJob select query.
func CollectionActionJobsQuery(app App) *dbx.SelectQuery {
	return app.AuxModelQuery(&CollectionActionJob{})
}

// FindCollectionActionJobById finds a single CollectionActionJob entry by its id.
func FindCollectionActionJobById(app App, id string) (*CollectionActionJob, error) {
	model := &CollectionActionJob{}

	err := CollectionActionJobsQuery(app).
		AndWhere(dbx.HashExp{"id": id}).
		Limit(1).
		One(model)

	if err != nil {
		return nil, err
	}

	return model, nil
}

// FindPendingCollectionActionJobs finds queued or running jobs.
func FindPendingCollectionActionJobs(app App, limit int) ([]*CollectionActionJob, error) {
	models := []*CollectionActionJob{}

	query := CollectionActionJobsQuery(app).AndWhere(dbx.In("status", CollectionActionJobStatusQueued, CollectionActionJobStatusRunning))
	if limit > 0 {
		query.Limit(int64(limit))
	}

	err := query.OrderBy("created").All(&models)
	return models, err
}

// FindQueuedCollectionActionJobs finds queued jobs that can be resumed after restart.
func FindQueuedCollectionActionJobs(app App, limit int) ([]*CollectionActionJob, error) {
	models := []*CollectionActionJob{}

	query := CollectionActionJobsQuery(app).AndWhere(dbx.HashExp{
		"status": CollectionActionJobStatusQueued,
	})
	if limit > 0 {
		query.Limit(int64(limit))
	}

	err := query.OrderBy("created").All(&models)
	return models, err
}

// RecoverCollectionActionJobs marks interrupted running jobs as failed and resumes queued jobs.
func RecoverCollectionActionJobs(app App) error {
	runningJobs := []*CollectionActionJob{}
	err := CollectionActionJobsQuery(app).
		AndWhere(dbx.HashExp{"status": CollectionActionJobStatusRunning}).
		OrderBy("created").
		All(&runningJobs)
	if err != nil {
		return err
	}

	for _, job := range runningJobs {
		job.Status = CollectionActionJobStatusFailed
		job.Error = "job interrupted by process restart"
		job.TouchFinished()
		_ = app.AuxSave(job)
	}

	queuedJobs, err := FindQueuedCollectionActionJobs(app, 100)
	if err != nil {
		return err
	}

	for _, job := range queuedJobs {
		queuedJob := job
		routine.FireAndForget(func() {
			if err := RunCollectionActionJob(app, queuedJob.Id); err != nil {
				app.Logger().Error(
					"Failed to resume queued collection action job",
					"jobId", queuedJob.Id,
					"error", err.Error(),
				)
			}
		})
	}

	return nil
}

// RunCollectionActionJob executes the provided job by id.
func RunCollectionActionJob(app App, jobId string) error {
	job, err := FindCollectionActionJobById(app, jobId)
	if err != nil {
		return err
	}

	return executeCollectionActionJob(app, job)
}

func executeCollectionActionJob(app App, job *CollectionActionJob) error {
	if job == nil {
		return errors.New("job is nil")
	}

	collection, err := app.FindCachedCollectionByNameOrId(job.CollectionId)
	if err != nil || collection == nil {
		collection, err = app.FindCachedCollectionByNameOrId(job.CollectionName)
		if err != nil || collection == nil {
			return fmt.Errorf("missing action collection context")
		}
	}

	action, ok := app.CollectionActions().Resolve(collection, job.ActionName)
	if !ok || action == nil {
		return fmt.Errorf("missing or invalid collection action")
	}

	job.Status = CollectionActionJobStatusRunning
	job.StatusMessage = "Starting action"
	job.Error = ""
	job.TouchStarted()
	if err := app.AuxSave(job); err != nil {
		return err
	}

	selectedIds := slices.Clone(job.RecordIds)
	selectedRecords, err := LoadCollectionActionRecords(app, collection, action, selectedIds)
	if err != nil {
		job.Status = CollectionActionJobStatusFailed
		job.Error = err.Error()
		job.TouchFinished()
		_ = app.AuxSave(job)
		return err
	}

	result, execErr := RunCollectionActionHandler(app, collection, action, selectedIds, selectedRecords, job, payloadFromJob(job))
	if execErr != nil {
		job.Status = CollectionActionJobStatusFailed
		job.Error = execErr.Error()
		job.TouchFinished()
		_ = app.AuxSave(job)
		return execErr
	}

	if result != nil {
		job.Result, _ = types.ParseJSONRaw(result)
		if message, ok := result["message"].(string); ok && message != "" {
			job.StatusMessage = message
		}
	}

	if job.TotalItems > 0 && job.ProcessedItems == 0 {
		job.ProcessedItems = job.TotalItems
	}

	job.Status = CollectionActionJobStatusSucceeded
	job.Error = ""
	job.TouchFinished()
	if err := app.AuxSave(job); err != nil {
		return err
	}

	return nil
}

func payloadFromJob(job *CollectionActionJob) map[string]any {
	if job == nil || len(job.Payload) == 0 {
		return map[string]any{}
	}

	payload := map[string]any{}
	_ = json.Unmarshal(job.Payload, &payload)
	return payload
}

func LoadCollectionActionRecords(app App, collection *Collection, action *CollectionAction, ids []string) ([]*Record, error) {
	if action == nil || !action.LoadRecords {
		return nil, nil
	}

	records, err := app.FindRecordsByIds(collection, ids)
	if err != nil {
		return nil, err
	}

	if len(records) != len(ids) {
		return nil, errors.New("one or more selected records no longer exist")
	}

	return records, nil
}

func RunCollectionActionHandler(
	app App,
	collection *Collection,
	action *CollectionAction,
	recordIds []string,
	records []*Record,
	job *CollectionActionJob,
	payload map[string]any,
) (result map[string]any, err error) {
	event := &CollectionActionRequestEvent{
		App: app,
		baseCollectionEventData: baseCollectionEventData{
			Collection: collection,
		},
		Action:    action,
		RecordIds: recordIds,
		Records:   records,
		Payload:   payload,
		Job:       job,
	}

	if action.Handler == nil {
		return nil, errors.New("missing action handler")
	}

	if err = action.Handler(event); err != nil {
		return nil, err
	}

	if event.Result != nil {
		return map[string]any{
			"message":        event.Result.Message,
			"data":           event.Result.Data,
			"clearSelection": event.Result.ClearSelection,
			"reloadRecords":  event.Result.ReloadRecords,
		}, nil
	}

	return map[string]any{
		"message": fmt.Sprintf("%s completed successfully.", action.Label),
	}, nil
}

func EnqueueCollectionActionJob(app App, collection *Collection, action *CollectionAction, recordIds []string, payload map[string]any) (*CollectionActionJob, error) {
	payloadRaw, err := types.ParseJSONRaw(payload)
	if err != nil {
		return nil, err
	}

	job := &CollectionActionJob{
		ActionName:     action.Name,
		ActionLabel:    action.Label,
		CollectionId:   collection.Id,
		CollectionName: collection.Name,
		Status:         CollectionActionJobStatusQueued,
		RecordIds:      append(types.JSONArray[string]{}, recordIds...),
		Payload:        payloadRaw,
		Result:         types.JSONRaw("{}"),
		ProcessedItems: 0,
		TotalItems:     len(recordIds),
	}

	if job.Id == "" {
		job.Id = GenerateDefaultRandomId()
	}

	if err := app.AuxSave(job); err != nil {
		return nil, err
	}

	return job, nil
}

func ValidateCollectionActionSelection(action *CollectionAction, ids []string) error {
	if action == nil {
		return errors.New("missing action")
	}

	if action.SelectionRequired && len(ids) == 0 {
		return errors.New("at least one selected record is required")
	}

	if action.MinSelection > 0 && len(ids) < action.MinSelection {
		return fmt.Errorf("at least %d selected records are required", action.MinSelection)
	}

	if action.MaxSelection > 0 && len(ids) > action.MaxSelection {
		return fmt.Errorf("a maximum of %d selected records are allowed", action.MaxSelection)
	}

	return nil
}

func NormalizeCollectionActionSelectedIds(ids []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(cast.ToString(id))
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}

	return result
}
