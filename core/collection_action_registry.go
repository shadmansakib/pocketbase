package core

import (
	"fmt"
	"sort"
	"sync"
)

type CollectionActionRegistry struct {
	mu      sync.RWMutex
	actions map[string]*CollectionAction
}

func NewCollectionActionRegistry() *CollectionActionRegistry {
	return &CollectionActionRegistry{
		actions: map[string]*CollectionAction{},
	}
}

func (r *CollectionActionRegistry) Add(action *CollectionAction) error {
	if r == nil {
		return fmt.Errorf("registry is nil")
	}

	if err := normalizeCollectionAction(action); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.actions[action.Name] = cloneCollectionAction(action)
	r.actions[action.Name].Handler = action.Handler

	return nil
}

func (r *CollectionActionRegistry) Remove(name string) {
	if r == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.actions, name)
}

func (r *CollectionActionRegistry) Resolve(collection *Collection, name string) (*CollectionAction, bool) {
	if r == nil {
		return nil, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	action, ok := r.actions[name]
	if !ok || !collectionActionApplies(action, collection) {
		return nil, false
	}

	return action, true
}

func (r *CollectionActionRegistry) List(collection *Collection) []*CollectionAction {
	if r == nil {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*CollectionAction, 0, len(r.actions))
	for _, action := range r.actions {
		if !collectionActionApplies(action, collection) {
			continue
		}

		result = append(result, cloneCollectionAction(action))
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Order != result[j].Order {
			return result[i].Order < result[j].Order
		}
		if result[i].Label != result[j].Label {
			return result[i].Label < result[j].Label
		}
		return result[i].Name < result[j].Name
	})

	return result
}
