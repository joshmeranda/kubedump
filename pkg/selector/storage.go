package selector

import "kubedump/pkg/filter"

type Store interface {
	// AddResource adds the given resource with the associated LabelSelector to the Store.
	AddResource(id ResourceId, labels LabelSelector)

	// GetResources fetches all resources whose LabelSelector matches the given Labels.
	GetResources(labels Labels) []ResourceId

	// RemoveResource deletes the given resourceId from storage. If no matching id exists, it will do nothing.
	RemoveResource(id ResourceId)
}

// NewStore constructs a store using the default implementation.
func NewStore() (Store, error) {
	return &memoryStore{
		inner: make(map[ResourceId]filter.Expression),
	}, nil
}

type memoryStore struct {
	inner map[ResourceId]filter.Expression
}

func (store *memoryStore) AddResource(id ResourceId, labels LabelSelector) {
	store.inner[id] = filter.NewLabelExpression(labels)
}

func (store *memoryStore) GetResources(labels Labels) []ResourceId {
	ids := make([]ResourceId, 0, len(store.inner))

	for id, expr := range store.inner {
		if expr.Matches(labels) {
			ids = append(ids, id)
		}
	}

	return ids
}

func (store *memoryStore) RemoveResource(id ResourceId) {
	delete(store.inner, id)
}
