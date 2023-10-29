package kubedump

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
