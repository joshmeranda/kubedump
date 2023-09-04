package kubedump

import (
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apibatchv1 "k8s.io/api/batch/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResourceFromFile(t *testing.T) {
	filePath := filepath.Join("..", "tests", "resources", "Service.dump", "default", "Pod", "sample-pod", "sample-pod.yaml")

	resource, err := NewResourceFromFile(filePath)
	require.NoError(t, err)

	assert.Equal(t, "Pod/sample-pod", resource.String())
	assert.Equal(t, "sample-pod", resource.GetName())
	assert.Equal(t, "default", resource.GetNamespace())
	assert.Equal(t, "Pod", resource.GetKind())
}

func TestSandbox(t *testing.T) {
	filePath := path.Join("..", "test.yaml")

	resource, err := NewResourceFromFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "Pod/test-pod", resource.String())
}

func TestBuilderValidate(t *testing.T) {
	builder := NewResourcePathBuilder()
	assert.Error(t, builder.Validate())

	builder.WithName("name")
	assert.Error(t, builder.Validate())

	builder.WithBase("basePath")
	assert.Error(t, builder.Validate())

	builder.WithKind("kind")
	assert.NoError(t, builder.Validate())

	builder.WithNamespace("namespace")
	assert.NoError(t, builder.Validate())

	assert.Equal(t, path.Join("basePath", "namespace", "kind", "name"), builder.Build())
}

func TestBuilderWithParent(t *testing.T) {
	job := &apibatchv1.Job{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-job",
			Namespace: "default",
		},
		TypeMeta: apimetav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "v1",
		},
	}

	parent := NewResourceBuilder().
		FromObject(job.ObjectMeta).
		FromType(job.TypeMeta).
		Build()

	resourcePath := NewResourcePathBuilder().
		WithBase(string(filepath.Separator)).
		WithParentResource(parent).
		WithKind("Pod").
		WithName("sample-job-xxxx").Build()

	assert.Equal(t, "/default/Job/sample-job/Pod/sample-job-xxxx", resourcePath)
}

func TestBuilderWithFile(t *testing.T) {
	filePath := NewResourcePathBuilder().
		WithBase("/").
		WithNamespace("default").
		WithKind("some-kind").
		WithName("some-resource").
		WithFileName("file.yaml").
		Build()

	assert.Equal(t, "/default/some-kind/some-resource/file.yaml", filePath)
}

func TestBuilderWithParentWithNamespaceConflict(t *testing.T) {
	job := &apibatchv1.Job{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-job",
			Namespace: "default",
		},
		TypeMeta: apimetav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "v1",
		},
	}

	parent := NewResourceBuilder().
		FromObject(job.ObjectMeta).
		FromType(job.TypeMeta).
		Build()

	builder := NewResourcePathBuilder().
		WithBase("/").
		WithKind("Pod").
		WithName("sample-job-xxxx")

	withParentPath := builder.WithParentResource(parent).Build()
	assert.Equal(t, "/default/Job/sample-job/Pod/sample-job-xxxx", withParentPath)

	withNamespacePath := builder.WithNamespace("default").Build()
	assert.Equal(t, "/default/Pod/sample-job-xxxx", withNamespacePath)
}
