package core

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pocketbase/pocketbase/tools/hook"
)

type CollectionActionExecutionMode string

const (
	CollectionActionExecutionSync  CollectionActionExecutionMode = "sync"
	CollectionActionExecutionAsync CollectionActionExecutionMode = "async"
)

// CollectionActionDefinition describes a custom collection admin action.
//
// It is aliased to CollectionAction so the Go and JSVM surfaces share the
// same public name without duplicating the underlying fields.
type CollectionActionDefinition = CollectionAction

// CollectionAction defines a custom collection admin action and its runtime handler.
type CollectionAction struct {
	Name               string
	Label              string
	Description        string
	Icon               string
	Order              int
	Collections        []string
	ExcludeCollections []string
	SelectionRequired  bool
	MinSelection       int
	MaxSelection       int
	ConfirmText        string
	Variant            string
	ExecutionMode      CollectionActionExecutionMode
	LoadRecords        bool
	ClearSelection     bool
	ReloadRecords      bool
	Handler            func(e *CollectionActionRequestEvent) error `json:"-"`
}

// CollectionActionResult represents the outcome returned by a collection action handler.
type CollectionActionResult struct {
	Message        string `json:"message,omitempty"`
	Data           any    `json:"data,omitempty"`
	ClearSelection bool   `json:"clearSelection,omitempty"`
	ReloadRecords  bool   `json:"reloadRecords,omitempty"`
	Job            any    `json:"job,omitempty"`
}

// CollectionActionRequestEvent is passed to custom collection action handlers.
type CollectionActionRequestEvent struct {
	hook.Event

	App          App
	RequestEvent *RequestEvent
	baseCollectionEventData

	Action    *CollectionAction
	RecordIds []string
	Records   []*Record
	Payload   map[string]any
	Job       *CollectionActionJob
	Result    *CollectionActionResult
}

func (e *CollectionActionRequestEvent) SetProgress(processedItems, totalItems int, message string) error {
	if e.Job == nil {
		return fmt.Errorf("progress updates are only available for async collection action jobs")
	}

	e.Job.ProcessedItems = processedItems
	e.Job.TotalItems = totalItems
	e.Job.StatusMessage = message

	return e.App.AuxSave(e.Job)
}

func (e *CollectionActionRequestEvent) SetResultMessage(message string) {
	if e.Result == nil {
		e.Result = &CollectionActionResult{}
	}
	e.Result.Message = message
}

func cloneCollectionAction(action *CollectionAction) *CollectionAction {
	if action == nil {
		return nil
	}

	clone := *action
	clone.Collections = slices.Clone(action.Collections)
	clone.ExcludeCollections = slices.Clone(action.ExcludeCollections)
	clone.Handler = nil

	return &clone
}

func collectionActionApplies(action *CollectionAction, collection *Collection) bool {
	if action == nil || collection == nil {
		return false
	}

	if len(action.Collections) == 0 {
		return !containsString(action.ExcludeCollections, collection.Id) && !containsString(action.ExcludeCollections, collection.Name)
	}

	if containsString(action.ExcludeCollections, collection.Id) || containsString(action.ExcludeCollections, collection.Name) {
		return false
	}

	for _, item := range action.Collections {
		if item == collection.Id || item == collection.Name {
			return true
		}
	}

	return false
}

func normalizeCollectionAction(action *CollectionAction) error {
	if action == nil {
		return fmt.Errorf("action is nil")
	}

	action.Name = strings.TrimSpace(action.Name)
	action.Label = strings.TrimSpace(action.Label)
	action.Description = strings.TrimSpace(action.Description)
	action.Icon = strings.TrimSpace(action.Icon)
	action.ConfirmText = strings.TrimSpace(action.ConfirmText)
	action.Variant = strings.TrimSpace(action.Variant)
	action.ExecutionMode = CollectionActionExecutionMode(strings.TrimSpace(string(action.ExecutionMode)))

	if action.Name == "" {
		return fmt.Errorf("action name is required")
	}

	if action.Label == "" {
		action.Label = action.Name
	}

	if action.ExecutionMode == "" {
		action.ExecutionMode = CollectionActionExecutionSync
	}

	switch action.ExecutionMode {
	case CollectionActionExecutionSync, CollectionActionExecutionAsync:
	default:
		return fmt.Errorf("invalid action execution mode %q", action.ExecutionMode)
	}

	if action.MinSelection < 0 {
		action.MinSelection = 0
	}

	if action.MaxSelection < 0 {
		action.MaxSelection = 0
	}

	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
