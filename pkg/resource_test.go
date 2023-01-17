package kubedump

import (
	"github.com/stretchr/testify/assert"
	apibatchv1 "k8s.io/api/batch/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path"
	"path/filepath"
	"testing"
)

func TestBuilderValidate(t *testing.T) {
	builder := NewResourceDirBuilder()
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
	handledParent, _ := NewHandledResource(HandleAdd, &apibatchv1.Job{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-job",
			Namespace: "default",
		},
		TypeMeta: apimetav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "v1",
		},
	})

	resourcePath := NewResourceDirBuilder().
		WithBase(string(filepath.Separator)).
		WithParentResource(handledParent).
		WithKind("Pod").
		WithName("sample-job-xxxx").Build()

	assert.Equal(t, "/default/Job/sample-job/Pod/sample-job-xxxx", resourcePath)
}
