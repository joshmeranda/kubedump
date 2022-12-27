package controller

import (
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	"sync"
)

type pair[F any, S any] struct {
	first  F
	second S
}

type storePair pair[LabelMatcher, HandledResource]

// todo: handle when checking resource against it's own selectors.

type Store interface {
	// AddResource adds the given resource with the associated LabelSelector
	AddResource(resource HandledResource, matcher LabelMatcher) error

	// GetResources fetches all resources whose LabelSelector matches the given Labels.
	GetResources(labels Labels) ([]HandledResource, error)

	// RemoveResource deletes the given resourceId from storage. If no matching resource exists, it will do nothing.
	RemoveResource(resource HandledResource) error
}

// NewStore constructs a store using the default implementation.
func NewStore() Store {
	return &memoryStore{
		inner: make(map[types.UID]storePair),
	}
}

type memoryStore struct {
	innerMut sync.RWMutex
	inner    map[types.UID]storePair
}

func (store *memoryStore) AddResource(resource HandledResource, matcher LabelMatcher) error {
	store.innerMut.Lock()
	defer store.innerMut.Unlock()

	store.inner[resource.GetUID()] = storePair{
		first:  matcher,
		second: resource,
	}

	return nil
}

func (store *memoryStore) GetResources(labels Labels) ([]HandledResource, error) {
	resources := make([]HandledResource, 0)

	for _, p := range store.inner {
		if p.first.Matches(labels) {
			resources = append(resources, p.second)
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
