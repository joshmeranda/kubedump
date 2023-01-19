package controller

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubedump "kubedump/pkg"
	"path"
	"testing"
)

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
		Resource: nil,
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
		Resource: nil,
	}

	expectedLinkPath := "/ns/Job/some-job/Pod/some-pod"
	expectedRelativePath := "../../../Pod/some-pod"

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

	basePath := t.TempDir()

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
	}

	dumpPath := path.Join(basePath, resource.GetNamespace(), resource.Kind, resource.GetName(), resource.GetName()+".yaml")

	err := dumpResourceDescription(dumpPath, resource)
	assert.NoError(t, err)

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
	}

	err = dumpResourceDescription(dumpPath, resource)

	assert.NoError(t, err)

	dumpPath = path.Join(basePath, resource.GetNamespace(), resource.Kind, resource.GetName(), resource.GetName()+".yaml")

	data, err = ioutil.ReadFile(dumpPath)
	assert.NoError(t, err)

	expectedData = "F: 1.5\nI: 0\nS: something\n"
	assert.Equal(t, expectedData, string(data))
}
