package kubedump

import (
	"fmt"
	"os"
	"path"

	apiappsv1 "k8s.io/api/apps/v1"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type HandledResource struct {
	apimetav1.Object
	apimetav1.TypeMeta

	// Resource is the actual k8s resource value
	Resource interface{}
}

func NewHandledResource(obj interface{}) (HandledResource, error) {
	// todo: client-go informer seems to drop TypeMeta info, so we need to add that manually
	if resource, ok := obj.(*unstructured.Unstructured); ok {
		return HandledResource{
			Object: resource,
			TypeMeta: apimetav1.TypeMeta{
				Kind:       resource.GetKind(),
				APIVersion: resource.GetAPIVersion(),
			},
			Resource: obj,
		}, nil
	}

	return HandledResource{}, fmt.Errorf("couldn't create handled resource from object of type '%+T'", obj)
}

func NewHandledResourceFromFile(kind string, filePath string) (HandledResource, error) {
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

	handledResource, err := NewHandledResource(resource)
	if err != nil {
		return HandledResource{}, err
	}

	return handledResource, nil
}

func (resource HandledResource) String() string {
	return fmt.Sprintf("%s/%s/%s", resource.Kind, resource.GetNamespace(), resource.GetName())
}

// ResourcePathBuilder can be used to build the parent directories for collected resources.
type ResourcePathBuilder struct {
	basePath           string
	parentResourcePath string
	namespace          string
	name               string
	kind               string
	fileName           string
}

func NewResourcePathBuilder() *ResourcePathBuilder {
	return &ResourcePathBuilder{}
}

func (builder *ResourcePathBuilder) WithBase(basePath string) *ResourcePathBuilder {
	builder.basePath = basePath
	return builder
}

func (builder *ResourcePathBuilder) WithName(name string) *ResourcePathBuilder {
	builder.name = name
	return builder
}

// WithNamespace instructs the builder to place the other components under tha path of the specified resource, and will
// also ignore any value passed to WithParentResource.
func (builder *ResourcePathBuilder) WithNamespace(namespace string) *ResourcePathBuilder {
	builder.namespace = namespace
	builder.parentResourcePath = ""
	return builder
}

func (builder *ResourcePathBuilder) WithKind(kind string) *ResourcePathBuilder {
	builder.kind = kind
	return builder
}

// WithParentResource instructs the builder to place the other components under the path of the specified resource, and
// will also ignore any value passed to WithNamespace.
func (builder *ResourcePathBuilder) WithParentResource(resource HandledResource) *ResourcePathBuilder {
	builder.parentResourcePath = path.Join(resource.GetNamespace(), resource.Kind, resource.GetName())
	builder.namespace = ""
	return builder
}

func (builder *ResourcePathBuilder) WithResource(resource HandledResource) *ResourcePathBuilder {
	builder.namespace = resource.GetNamespace()
	builder.name = resource.GetName()
	builder.kind = resource.Kind
	return builder
}

func (builder *ResourcePathBuilder) WithFileName(fileName string) *ResourcePathBuilder {
	builder.fileName = fileName
	return builder
}

// Validate that the builder will be able to build a resource path.
func (builder *ResourcePathBuilder) Validate() error {
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

// Reset the state of the builder as if it was new.
func (builder *ResourcePathBuilder) Reset() {
	builder.basePath = ""
	builder.parentResourcePath = ""
	builder.namespace = ""
	builder.name = ""
	builder.kind = ""
}

// Build joins the different components of the ResourcePathBuilder and panics if any value (except namespace) is unset.
func (builder *ResourcePathBuilder) Build() string {
	if err := builder.Validate(); err != nil {
		panic(err)
	}

	return path.Join(builder.basePath, builder.parentResourcePath, builder.namespace, builder.kind, builder.name, builder.fileName)
}
