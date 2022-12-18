package selector

import (
	"fmt"
	"kubedump/pkg/filter"
	"reflect"
)

type Store interface {
	// AddResource adds the given resource with the associated LabelSelector
	AddResource(resource ResourceWrapper, labels LabelSelector) error

	// GetResources fetches all resources whose LabelSelector matches the given Labels.
	GetResources(labels Labels) ([]ResourceWrapper, error)

	// RemoveResource deletes the given resourceId from storage. If no matching resource exists, it will do nothing.
	RemoveResource(resource ResourceWrapper) error
}

// NewStore constructs a store using the default implementation.
func NewStore() (Store, error) {
	return &memoryStore{
		inner: make(map[ResourceWrapper]filter.Expression),
	}, nil
}

type memoryStore struct {
	inner map[ResourceWrapper]filter.Expression
}

func (store *memoryStore) AddResource(resource ResourceWrapper, labels LabelSelector) error {
	if reflect.ValueOf(resource.Resource).Kind() == reflect.Ptr {
		return fmt.Errorf("memoeryStore resource cannot be a pointer, received %T", resource.Resource)
	}

	store.inner[resource] = filter.NewLabelExpression(labels)

	return nil
}

func (store *memoryStore) GetResources(labels Labels) ([]ResourceWrapper, error) {
	ids := make([]ResourceWrapper, 0, len(store.inner))

	for id, expr := range store.inner {
		if expr.Matches(labels) {
			ids = append(ids, id)
		}
	}

	return ids, nil
}

func (store *memoryStore) RemoveResource(resource ResourceWrapper) error {
	delete(store.inner, resource)

	return nil
}
