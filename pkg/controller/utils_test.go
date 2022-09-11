package controller

import (
	"github.com/stretchr/testify/assert"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestResourceDirPathNamespaced(t *testing.T) {
	expected := "parent/test/pod/some-pod"
	actual := resourceDirPath("Pod", "parent", &apismetav1.ObjectMeta{
		Name:      "some-pod",
		Namespace: "test",
	})

	assert.Equal(t, expected, actual)

	// the owner should be ignored
	actual = resourceDirPath("Pod", "parent", &apismetav1.ObjectMeta{
		Name:      "some-pod",
		Namespace: "test",
		OwnerReferences: []apismetav1.OwnerReference{
			{
				Name: "owner-job",
				Kind: "Job",
			},
		},
	})

	assert.Equal(t, expected, actual)

	expected = "parent/test/job/some-job"
	actual = resourceDirPath("Job", "parent", &apismetav1.ObjectMeta{
		Name:      "some-job",
		Namespace: "test",
	})

	assert.Equal(t, expected, actual)
}

func TestResourceDirPathNonNamespaced(t *testing.T) {
	expected := "parent/node/some-node"
	actual := resourceDirPath("Node", "parent", &apismetav1.ObjectMeta{
		Name: "some-node",
	})

	assert.Equal(t, expected, actual)
}
