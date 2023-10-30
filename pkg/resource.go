package kubedump

import (
	"fmt"
	"os"
	"path"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

// todo: these builders don't need to be pointer receivers or returns

// Resource is a collection of methods that can be used to describe a resource being handled by the kubedump controller.
type Resource interface {
	fmt.Stringer

	GetName() string

	GetNamespace() string

	GetLabels() map[string]string

	GetOwnershipReferences() []apimetav1.OwnerReference

	GetKind() string

	GetUID() types.UID
}

func NewResourceFromFile(path string) (Resource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read resource file: %w", err)
	}

	m := map[string]interface{}{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("could not unmarshal data to suntructured: %w", err)
	}
	u := &unstructured.Unstructured{Object: m}

	return NewResourceBuilder().
		FromUnstructured(u).
		Build(), nil
}

type resource struct {
	name            string
	namespace       string
	labels          map[string]string
	ownerReferences []apimetav1.OwnerReference
	kind            string
	id              types.UID
}

func (resource *resource) String() string {
	return fmt.Sprintf("%s/%s", resource.GetKind(), resource.GetName())
}

func (resource *resource) GetName() string {
	return resource.name
}

func (resource *resource) GetNamespace() string {
	return resource.namespace
}

func (resource *resource) GetLabels() map[string]string {
	return resource.labels
}

func (resource *resource) GetOwnershipReferences() []apimetav1.OwnerReference {
	return resource.ownerReferences
}

func (resource *resource) GetKind() string {
	return resource.kind
}

func (resource *resource) GetUID() types.UID {
	return resource.id
}

type ResourceBuilder struct {
	resource resource
}

func NewResourceBuilder() *ResourceBuilder {
	return &ResourceBuilder{}
}

func (builder *ResourceBuilder) FromUnstructured(u *unstructured.Unstructured) *ResourceBuilder {
	builder.resource.name = u.GetName()
	builder.resource.namespace = u.GetNamespace()
	builder.resource.labels = u.GetLabels()
	builder.resource.ownerReferences = u.GetOwnerReferences()
	builder.resource.kind = u.GetKind()
	builder.resource.id = u.GetUID()
	return builder
}

func (builder *ResourceBuilder) FromObject(obj apimetav1.ObjectMeta) *ResourceBuilder {
	builder.resource.name = obj.Name
	builder.resource.namespace = obj.Namespace
	builder.resource.labels = obj.Labels
	builder.resource.ownerReferences = obj.OwnerReferences
	builder.resource.id = obj.UID
	return builder
}

func (builder *ResourceBuilder) FromType(t apimetav1.TypeMeta) *ResourceBuilder {
	builder.resource.kind = t.Kind
	return builder
}

func (builder *ResourceBuilder) WithName(name string) *ResourceBuilder {
	builder.resource.name = name
	return builder
}

func (builder *ResourceBuilder) WithNamespace(namespace string) *ResourceBuilder {
	builder.resource.namespace = namespace
	return builder
}

func (builder *ResourceBuilder) WithLabels(labels map[string]string) *ResourceBuilder {
	builder.resource.labels = labels
	return builder
}

func (builder *ResourceBuilder) WithOwnershipReferences(ownerReferences []apimetav1.OwnerReference) *ResourceBuilder {
	builder.resource.ownerReferences = ownerReferences
	return builder
}

func (builder *ResourceBuilder) WithKind(kind string) *ResourceBuilder {
	builder.resource.kind = kind
	return builder
}

func (builder *ResourceBuilder) WithId(id types.UID) *ResourceBuilder {
	builder.resource.id = id
	return builder
}

func (builder *ResourceBuilder) Build() Resource {
	return &builder.resource
}

// ResourcePathBuilder can be used to build the directory paths to a resource. You must build the paths to any resource files or subdirectories yourself.
type ResourcePathBuilder struct {
	BasePath  string
	Namespace string

	ParentName string
	ParentKind string

	Name string
	Kind string
}

func (builder ResourcePathBuilder) WithBase(basePath string) ResourcePathBuilder {
	builder.BasePath = basePath
	return builder
}

func (builder ResourcePathBuilder) WithNamespace(namespace string) ResourcePathBuilder {
	builder.Namespace = namespace
	return builder
}

func (builder ResourcePathBuilder) WithName(name string) ResourcePathBuilder {
	builder.Name = name
	return builder
}

func (builder ResourcePathBuilder) WithKind(kind string) ResourcePathBuilder {
	builder.Kind = kind
	return builder
}

func (builder ResourcePathBuilder) WithParentName(name string) ResourcePathBuilder {
	builder.ParentName = name
	return builder
}

func (builder ResourcePathBuilder) WithParentKind(kind string) ResourcePathBuilder {
	builder.ParentKind = kind
	return builder
}

func (builder ResourcePathBuilder) WithResource(resource Resource) ResourcePathBuilder {
	builder.Namespace = resource.GetNamespace()
	builder.Name = resource.GetName()
	builder.Kind = resource.GetKind()
	return builder
}

func (builder ResourcePathBuilder) BuildNamespace() string {
	return path.Join(builder.BasePath, builder.Namespace)
}

func (builder ResourcePathBuilder) BuildKind() string {
	return path.Join(builder.BasePath, builder.Namespace, builder.Kind)
}

func (builder ResourcePathBuilder) Build() string {
	return path.Join(builder.BasePath, builder.Namespace, builder.Kind, builder.Name)
}

func (builder ResourcePathBuilder) BuildWithParent() string {
	return path.Join(builder.BasePath, builder.Namespace, builder.ParentKind, builder.ParentName, builder.Kind, builder.Name)
}
