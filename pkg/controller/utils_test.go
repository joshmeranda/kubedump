package controller

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubedump "kubedump/pkg"
	"os"
	"path"
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

func TestGetSymlinkPaths(t *testing.T) {
	parent := kubedump.HandledResource{
		Object: &apimetav1.ObjectMeta{
			Name:      "some-job",
			Namespace: "ns",
		},
		TypeMeta: apimetav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "v1",
		},
		Resource:        nil,
		HandleEventKind: "Job",
	}

	child := kubedump.HandledResource{
		Object: &apimetav1.ObjectMeta{
			Name:      "some-pod",
			Namespace: "ns",
		},
		TypeMeta: apimetav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		Resource:        nil,
		HandleEventKind: "Pod",
	}

	expectedLinkPath := "/ns/job/some-job/pod/some-pod"
	expectedRelativePath := "../../../pod/some-pod"

	linkPath, relativePath, err := getSymlinkPaths("/", parent, child)

	assert.NoError(t, err)
	assert.Equal(t, expectedLinkPath, linkPath)
	assert.Equal(t, expectedRelativePath, relativePath)
}

func TestDumpResource(t *testing.T) {
	type DummyStruct struct {
		I int
		F float64
		S string
	}

	basePath, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatalf("could not create temporary file")
	}
	defer os.RemoveAll(basePath)

	resource := kubedump.HandledResource{
		Object: &apimetav1.ObjectMeta{
			Name:      "some-resource",
			Namespace: "ns",
		},
		TypeMeta: apimetav1.TypeMeta{
			Kind:       "nothing",
			APIVersion: "v0",
		},
		Resource: DummyStruct{
			I: 0,
			F: 1.5,
			S: "something longer than the next",
		},
		HandleEventKind: kubedump.HandleAdd,
	}

	err = dumpResourceDescription(basePath, resource)

	assert.NoError(t, err)

	dumpPath := path.Join(basePath, resource.GetNamespace(), resource.Kind, resource.GetName(), resource.GetName()+".yaml")

	data, err := ioutil.ReadFile(dumpPath)
	assert.NoError(t, err)

	expectedData := "F: 1.5\nI: 0\nS: something longer than the next\n"
	assert.Equal(t, expectedData, string(data))

	// test overwriting file
	resource = kubedump.HandledResource{
		Object: &apimetav1.ObjectMeta{
			Name:      "some-resource",
			Namespace: "ns",
		},
		TypeMeta: apimetav1.TypeMeta{
			Kind:       "nothing",
			APIVersion: "v0",
		},
		Resource: DummyStruct{
			I: 0,
			F: 1.5,
			S: "something",
		},
		HandleEventKind: kubedump.HandleAdd,
	}

	err = dumpResourceDescription(basePath, resource)

	assert.NoError(t, err)

	dumpPath = path.Join(basePath, resource.GetNamespace(), resource.Kind, resource.GetName(), resource.GetName()+".yaml")

	data, err = ioutil.ReadFile(dumpPath)
	assert.NoError(t, err)

	expectedData = "F: 1.5\nI: 0\nS: something\n"
	assert.Equal(t, expectedData, string(data))
}
