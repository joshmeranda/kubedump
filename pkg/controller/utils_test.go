package controller

import (
	"github.com/stretchr/testify/assert"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestResourceDirPathNamespaced(t *testing.T) {
	expected := "parent/test/pod/some-pod"
	actual := resourceDirPath("parent", "Pod", &apimetav1.ObjectMeta{
		Name:      "some-pod",
		Namespace: "test",
	})

	assert.Equal(t, expected, actual)

	// the owner should be ignored
	actual = resourceDirPath("parent", "Pod", &apimetav1.ObjectMeta{
		Name:      "some-pod",
		Namespace: "test",
		OwnerReferences: []apimetav1.OwnerReference{
			{
				Name: "owner-job",
				Kind: "Job",
			},
		},
	})

	assert.Equal(t, expected, actual)

	expected = "parent/test/job/some-job"
	actual = resourceDirPath("parent", "Job", &apimetav1.ObjectMeta{
		Name:      "some-job",
		Namespace: "test",
	})

	assert.Equal(t, expected, actual)
}

func TestResourceDirPathNonNamespaced(t *testing.T) {
	expected := "parent/node/some-node"
	actual := resourceDirPath("parent", "Node", &apimetav1.ObjectMeta{
		Name: "some-node",
	})

	assert.Equal(t, expected, actual)
}
