package core

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/pocketbase/pocketbase/tools/hook"
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
	Result    *CollectionActionResult
}

// SetResultMessage initializes the action result if needed and sets its message.
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

	if action.Name == "" {
		return fmt.Errorf("action name is required")
	}

	if action.Label == "" {
		action.Label = action.Name
	}

	if action.MinSelection < 0 {
		action.MinSelection = 0
	}

	if action.MaxSelection < 0 {
		action.MaxSelection = 0
	}

	return nil
}

// LoadCollectionActionRecords loads the selected records when the action asks for record models.
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

// RunCollectionActionHandler executes a registered collection action handler.
func RunCollectionActionHandler(
	app App,
	collection *Collection,
	action *CollectionAction,
	recordIds []string,
	records []*Record,
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

// ValidateCollectionActionSelection checks the selected record ids against an action definition.
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

// NormalizeCollectionActionSelectedIds trims, drops empty values, and removes duplicate selected ids.
func NormalizeCollectionActionSelectedIds(ids []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
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

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
