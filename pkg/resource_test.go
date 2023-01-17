package kubedump

import (
	"github.com/stretchr/testify/assert"
	"path"
	"testing"
)

func TestValidate(t *testing.T) {
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
