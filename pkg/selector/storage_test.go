package selector

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAddPPointer(t *testing.T) {
	store, err := NewStore()

	assert.NoError(t, err)

	n := 10
	var v interface{} = n
	var vPtr interface{} = &n

	err = store.AddResource(ResourceWrapper{Resource: v}, LabelSelector{})
	assert.NoError(t, err)

	err = store.AddResource(ResourceWrapper{Resource: vPtr}, LabelSelector{})
	assert.Error(t, err)
}
