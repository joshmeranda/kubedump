package kubedump

import (
	"fmt"
	apiappsv1 "k8s.io/api/apps/v1"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
	apieventsv1 "k8s.io/api/events/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type HandleKind string

const (
	HandleAdd    HandleKind = "Add"
	HandleUpdate            = "Edit"
	HandleDelete            = "Delete"
)

type HandledResource struct {
	apimetav1.Object
	apimetav1.TypeMeta

	// Resource is the actual k8s resource value
	Resource interface{}

	// HandleEventKind is the type of handler event which sourced this Resource.
	HandleEventKind HandleKind
}

func NewHandledResource(handledKind HandleKind, obj interface{}) (HandledResource, error) {
	// todo: client-go informer seems to drop TypeMeta info, so we need to add that manually for now
	switch resource := obj.(type) {
	case *apieventsv1.Event:
		return HandledResource{
			Object: resource,
			TypeMeta: apimetav1.TypeMeta{
				Kind:       "Event",
				APIVersion: "v1",
			},
			Resource:        resource,
			HandleEventKind: handledKind,
		}, nil
	case *apicorev1.Pod:
		return HandledResource{
			Object: resource,
			TypeMeta: apimetav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			Resource:        resource,
			HandleEventKind: handledKind,
		}, nil
	case *apicorev1.Service:
		return HandledResource{
			Object: resource,
			TypeMeta: apimetav1.TypeMeta{
				Kind:       "Service",
				APIVersion: "v1",
			},
			Resource:        resource,
			HandleEventKind: handledKind,
		}, nil
	case *apibatchv1.Job:
		return HandledResource{
			Object: resource,
			TypeMeta: apimetav1.TypeMeta{
				Kind:       "Job",
				APIVersion: "batch/v1",
			},
			Resource:        resource,
			HandleEventKind: handledKind,
		}, nil
	case *apiappsv1.ReplicaSet:
		return HandledResource{
			Object: resource,
			TypeMeta: apimetav1.TypeMeta{
				Kind:       "ReplicaSet",
				APIVersion: "apps/1",
			},
			Resource:        resource,
			HandleEventKind: handledKind,
		}, nil
	case *apiappsv1.Deployment:
		return HandledResource{
			Object: resource,
			TypeMeta: apimetav1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
			},
			Resource:        resource,
			HandleEventKind: handledKind,
		}, nil
	case *apicorev1.ConfigMap:
		return HandledResource{
			Object: resource,
			TypeMeta: apimetav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			Resource:        resource,
			HandleEventKind: handledKind,
		}, nil
	default:
		return HandledResource{}, fmt.Errorf("value of type '%F' cannot be a HandledResource", obj)
	}
}

func (resource HandledResource) String() string {
	return fmt.Sprintf("%s/%s/%s", resource.Kind, resource.GetNamespace(), resource.GetName())
}
