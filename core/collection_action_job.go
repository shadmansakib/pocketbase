package core

import (
	"context"

	"github.com/pocketbase/pocketbase/tools/types"
)

const CollectionActionJobsTableName = "_action_jobs"

const (
	CollectionActionJobStatusQueued    = "queued"
	CollectionActionJobStatusRunning   = "running"
	CollectionActionJobStatusSucceeded = "succeeded"
	CollectionActionJobStatusFailed    = "failed"
)

var _ Model = (*CollectionActionJob)(nil)

type CollectionActionJob struct {
	BaseModel

	ActionName     string                  `db:"actionName" json:"actionName"`
	ActionLabel    string                  `db:"actionLabel" json:"actionLabel"`
	CollectionId   string                  `db:"collectionId" json:"collectionId"`
	CollectionName string                  `db:"collectionName" json:"collectionName"`
	Status         string                  `db:"status" json:"status"`
	RecordIds      types.JSONArray[string] `db:"recordIds" json:"recordIds"`
	Payload        types.JSONRaw           `db:"payload" json:"payload"`
	Result         types.JSONRaw           `db:"result" json:"result"`
	Error          string                  `db:"error" json:"error"`
	ProcessedItems int                     `db:"processedItems" json:"processedItems"`
	TotalItems     int                     `db:"totalItems" json:"totalItems"`
	StatusMessage  string                  `db:"statusMessage" json:"statusMessage"`
	Started        types.DateTime          `db:"started" json:"started"`
	Finished       types.DateTime          `db:"finished" json:"finished"`
	Created        types.DateTime          `db:"created" json:"created"`
	Updated        types.DateTime          `db:"updated" json:"updated"`
}

func (m *CollectionActionJob) TableName() string {
	return CollectionActionJobsTableName
}

func (m *CollectionActionJob) TouchStarted() {
	m.Started = types.NowDateTime()
}

func (m *CollectionActionJob) TouchFinished() {
	m.Finished = types.NowDateTime()
}

// DBExport prepares and exports the current job for db persistence.
func (m *CollectionActionJob) DBExport(app App) (map[string]any, error) {
	now := types.NowDateTime()

	result := map[string]any{
		"id":             m.PK(),
		"actionName":     m.ActionName,
		"actionLabel":    m.ActionLabel,
		"collectionId":   m.CollectionId,
		"collectionName": m.CollectionName,
		"status":         m.Status,
		"recordIds":      m.RecordIds,
		"payload":        m.Payload,
		"result":         m.Result,
		"error":          m.Error,
		"processedItems": m.ProcessedItems,
		"totalItems":     m.TotalItems,
		"statusMessage":  m.StatusMessage,
		"started":        m.Started,
		"finished":       m.Finished,
	}

	if len(m.Result) == 0 {
		result["result"] = types.JSONRaw("{}")
	}

	if m.IsNew() {
		result["created"] = now
	}
	result["updated"] = now

	return result, nil
}

// PostValidate implements [PostValidator] and marks the model as persisted after scans.
func (m *CollectionActionJob) PostValidate(ctx context.Context, app App) error {
	return nil
}
