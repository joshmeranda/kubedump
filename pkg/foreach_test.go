package kubedump

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var dumpDir = filepath.Join("..", "tests", "resources", "LinkedService.dump")

func TestForEachNamespace(t *testing.T) {
	namespaces := []string{}
	err := ForEachNamespace(dumpDir, func(builder ResourcePathBuilder) error {
		namespaces = append(namespaces, builder.Namespace)
		return nil
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"default"}, namespaces)
}

func TestForEachKind(t *testing.T) {
	kinds := []string{}
	err := ForEachKind(dumpDir, func(builder ResourcePathBuilder) error {
		kinds = append(kinds, builder.Kind)
		return nil
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"Service", "Pod", "ConfigMap", "Secret"}, kinds)
}

func TestForEachResource(t *testing.T) {
	resources := []string{}
	err := ForEachResource(dumpDir, func(builder ResourcePathBuilder) error {
		resources = append(resources, fmt.Sprintf("%s:%s/%s", builder.Kind, builder.Namespace, builder.Name))
		return nil
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"Service:default/sample-service", "Pod:default/sample-pod", "ConfigMap:default/sample-configmap", "Secret:default/sample-secret"}, resources)
}
