package controller

import (
	"fmt"
	"sync"

	kubedump "github.com/joshmeranda/kubedump/pkg"
	"k8s.io/apimachinery/pkg/types"
)

type pair[F any, S any] struct {
	first  F
	second S
}

type storePair pair[Matcher, kubedump.Resource]

type Store interface {
	// AddResource adds the given resource with the associated LabelSelector
	AddResource(resource kubedump.Resource, matcher Matcher) error

	// GetResources fetches all resources whose LabelSelector matches the given Labels.
	//
	// Note: if the given resource matches a selector, and they are the same k8s object, that selector should not be
	// included in the returned slice.
	GetResources(resource kubedump.Resource) ([]kubedump.Resource, error)

	// RemoveResource deletes the given resourceId from storage. If no matching resource exists, it will do nothing.
	RemoveResource(resource kubedump.Resource) error
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

func (store *memoryStore) AddResource(resource kubedump.Resource, matcher Matcher) error {
	store.innerMut.Lock()
	defer store.innerMut.Unlock()

	store.inner[resource.GetUID()] = storePair{
		first:  matcher,
		second: resource,
	}

	return nil
}

func (store *memoryStore) GetResources(resource kubedump.Resource) ([]kubedump.Resource, error) {
	resources := make([]kubedump.Resource, 0)

	store.innerMut.RLock()
	defer store.innerMut.RUnlock()

	for _, p := range store.inner {
		if p.second.GetUID() == resource.GetUID() || p.second.GetNamespace() != resource.GetNamespace() {
			continue
		}

		if p.first.Matches(resource) {
			resources = append(resources, p.second)
		}
	}

	return resources, nil
}

func (store *memoryStore) RemoveResource(resource kubedump.Resource) error {
	_, found := store.inner[resource.GetUID()]

	if !found {
		return fmt.Errorf("store does not contain any %s %s/%s", resource.GetKind(), resource.GetNamespace(), resource.GetName())
	}

	delete(store.inner, resource.GetUID())

	return nil
}
