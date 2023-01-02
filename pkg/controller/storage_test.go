package controller

import (
	"github.com/stretchr/testify/assert"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubedump "kubedump/pkg"
	"testing"
)

var serviceWithEmptySelector = &apicorev1.Service{
	TypeMeta:   apimetav1.TypeMeta{},
	ObjectMeta: apimetav1.ObjectMeta{},
	Spec: apicorev1.ServiceSpec{
		Selector: map[string]string{},
	},
	Status: apicorev1.ServiceStatus{},
}

var podWithLabels = &apicorev1.Pod{
	TypeMeta: apimetav1.TypeMeta{},
	ObjectMeta: apimetav1.ObjectMeta{
		Labels: map[string]string{
			"some-label": "some-value",
		},
		UID: "manually-entered-and-invalid-uid",
	},
	Spec:   apicorev1.PodSpec{},
	Status: apicorev1.PodStatus{},
}

func TestServiceEmptyMatcher(t *testing.T) {
	store := NewStore()

	resource, err := kubedump.NewHandledResource(kubedump.HandleAdd, serviceWithEmptySelector)
	assert.NoError(t, err)

	matcher, err := selectorFromHandled(resource)
	assert.NoError(t, err)

	err = store.AddResource(resource, matcher)
	assert.NoError(t, err)

	resource, err = kubedump.NewHandledResource(kubedump.HandleAdd, podWithLabels)
	assert.NoError(t, err)

	resources, err := store.GetResources(resource)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(resources))

	for _, r := range resources {
		t.Log(r.String())
	}
}
