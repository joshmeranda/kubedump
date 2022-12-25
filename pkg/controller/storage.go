package controller

import (
	"fmt"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sync"
)

type Store interface {
	// AddResource adds the given resource with the associated LabelSelector
	AddResource(resource HandledResource, selector labels.Selector) error

	// GetResources fetches all resources whose LabelSelector matches the given Labels.
	GetResources(labels labels.Labels) ([]HandledResource, error)

	// RemoveResource deletes the given resourceId from storage. If no matching resource exists, it will do nothing.
	RemoveResource(resource HandledResource) error
}

// NewStore constructs a store using the default implementation.
func NewStore() (Store, error) {
	return &memoryStore{
		inner: make(map[types.UID]pair),
	}, nil
}

type pair struct {
	selector labels.Selector
	resource HandledResource
}

type memoryStore struct {
	innerMut sync.RWMutex
	inner    map[types.UID]pair
}

func (store *memoryStore) AddResource(resource HandledResource, selector labels.Selector) error {
	store.innerMut.Lock()
	defer store.innerMut.Unlock()

	store.inner[resource.GetUID()] = pair{
		selector: selector,
		resource: resource,
	}

	return nil
}

func (store *memoryStore) GetResources(labels labels.Labels) ([]HandledResource, error) {
	resources := make([]HandledResource, 0)

	for _, p := range store.inner {
		if p.selector.Matches(labels) {
			resources = append(resources, p.resource)
		}
	}

	return resources, nil
}

func (store *memoryStore) RemoveResource(resource HandledResource) error {
	_, found := store.inner[resource.GetUID()]

	if !found {
		return fmt.Errorf("store does not contain any %s %s/%s", resource.Kind, resource.GetNamespace(), resource.GetName())
	}

	delete(store.inner, resource.GetUID())

	return nil
}
