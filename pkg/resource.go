package kubedump

import (
	"fmt"
	apiappsv1 "k8s.io/api/apps/v1"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
	apieventsv1 "k8s.io/api/events/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path"
	"sigs.k8s.io/yaml"
	"strings"
)

type HandleKind string

const (
	HandleAdd    HandleKind = "Add"
	HandleUpdate            = "Edit"
	HandleDelete            = "Delete"
	HandleFilter            = "filter"
)

type HandledResource struct {
	apimetav1.Object
	apimetav1.TypeMeta

	// Resource is the actual k8s resource value
	Resource interface{}

	// todo: we might want to decouple this from HandledResource and leave it to the informer handlers to inform the
	//       Handler function what type of event is being received
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
	case *apicorev1.Secret:
		return HandledResource{
			Object: resource,
			TypeMeta: apimetav1.TypeMeta{
				Kind:       "Secret",
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

func NewHandledResourceFromFile(handledKind HandleKind, kind string, filePath string) (HandledResource, error) {
	var resource interface{}

	switch kind {
	case "Pod":
		var inner apicorev1.Pod
		resource = &inner
	case "Service":
		var inner apicorev1.Service
		resource = &inner
	case "Secret":
		var inner apicorev1.Secret
		resource = &inner
	case "ConfigMap":
		var inner apicorev1.ConfigMap
		resource = &inner
	case "Job":
		var inner apibatchv1.Job
		resource = &inner
	case "ReplicaSet":
		var inner apiappsv1.ReplicaSet
		resource = &inner
	case "Deployment":
		var inner apiappsv1.Deployment
		resource = &inner
	default:
		return HandledResource{}, fmt.Errorf("unhandled kind '%s'", kind)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return HandledResource{}, fmt.Errorf("error reading from file '%s': %w", filePath, err)
	}

	if err := yaml.Unmarshal(data, resource); err != nil {
		return HandledResource{}, fmt.Errorf("error unmarshailng resource of kind '%s': %w", kind, err)
	}

	handledResource, err := NewHandledResource(handledKind, resource)
	if err != nil {
		return HandledResource{}, err
	}

	return handledResource, nil
}

func (resource HandledResource) String() string {
	return fmt.Sprintf("%s/%s/%s", resource.Kind, resource.GetNamespace(), resource.GetName())
}

// ResourceDirBuilder can be used to build the parent directories for collected resources.
type ResourceDirBuilder struct {
	basePath  string
	namespace string
	name      string
	kind      string
}

func NewResourceDirBuilder() *ResourceDirBuilder {
	return &ResourceDirBuilder{}
}

func (builder *ResourceDirBuilder) WithBase(basePath string) *ResourceDirBuilder {
	builder.basePath = basePath
	return builder
}

func (builder *ResourceDirBuilder) WithName(name string) *ResourceDirBuilder {
	builder.name = name
	return builder
}

func (builder *ResourceDirBuilder) WithNamespace(namespace string) *ResourceDirBuilder {
	builder.namespace = namespace
	return builder
}

func (builder *ResourceDirBuilder) WithKind(kind string) *ResourceDirBuilder {
	builder.kind = kind
	return builder
}

func (builder *ResourceDirBuilder) WithObject(obj apimetav1.Object) *ResourceDirBuilder {
	builder.name = obj.GetName()
	builder.namespace = obj.GetNamespace()
	return builder
}

func (builder *ResourceDirBuilder) WithType(t apimetav1.TypeMeta) *ResourceDirBuilder {
	builder.kind = strings.ToLower(t.Kind)
	return builder
}

func (builder *ResourceDirBuilder) WithResource(resource HandledResource) *ResourceDirBuilder {
	return builder.WithObject(resource).WithType(resource.TypeMeta)
}

func (builder *ResourceDirBuilder) WithBuilder(otherBuilder *ResourceDirBuilder) *ResourceDirBuilder {
	return builder.WithBase(otherBuilder.basePath).WithNamespace(otherBuilder.namespace).WithName(otherBuilder.name).WithKind(otherBuilder.kind)
}

func (builder *ResourceDirBuilder) Validate() error {
	if builder.basePath == "" {
		return fmt.Errorf("basePath must be set")
	}

	if builder.name == "" {
		return fmt.Errorf("name must be set")
	}

	if builder.kind == "" {
		return fmt.Errorf("kind must be set")
	}

	return nil
}

// Build joins the different components of the ResourceDirBuilder and panics if any value (except namespace) is unset.
func (builder *ResourceDirBuilder) Build() string {
	if err := builder.Validate(); err != nil {
		panic(err)
	}

	if builder.namespace == "" {
		// todo: this is subject to change (and probably should)
		return path.Join(builder.basePath, builder.kind, builder.name)
	}

	return path.Join(builder.basePath, builder.namespace, builder.kind, builder.name)
}
