package controller

import (
	"os"
	"path"
	"testing"

	kubedump "github.com/joshmeranda/kubedump/pkg"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

/*
func TestGetSymlinkPaths(t *testing.T) {
	parent := kubedump.NewResourceBuilder().
		FromObject(apimetav1.ObjectMeta{
			Name:      "some-job",
			Namespace: "ns",
		}).
		FromType(apimetav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "v1",
		}).
		Build()

	child := kubedump.NewResourceBuilder().
		FromObject(apimetav1.ObjectMeta{
			Name:      "some-pod",
			Namespace: "ns",
		}).
		FromType(apimetav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		}).
		Build()

	expectedLinkPath := "/ns/Job/some-job/Pod/some-pod"
	expectedRelativePath := "../../../Pod/some-pod"

	linkPath, relativePath, err := getSymlinkPaths("/", parent, child)

	assert.NoError(t, err)
	assert.Equal(t, expectedLinkPath, linkPath)
	assert.Equal(t, expectedRelativePath, relativePath)
}
*/

func TestDumpResource(t *testing.T) {
	basePath := t.TempDir()

	u := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}
	u.SetKind("nothing")
	u.SetName("some-resource")
	u.SetNamespace("ns")

	resource := kubedump.NewResourceBuilder().
		FromUnstructured(u).
		Build()

	dumpPath := path.Join(basePath, resource.GetNamespace(), resource.GetKind(), resource.GetName(), resource.GetName()+".yaml")

	err := dumpResourceDescription(dumpPath, u)
	assert.NoError(t, err)

	data, err := os.ReadFile(dumpPath)
	assert.NoError(t, err)

	expectedData := "kind: nothing\nmetadata:\n  name: some-resource\n  namespace: ns\n"
	assert.Equal(t, expectedData, string(data))

	// test overwriting file
	u.SetName("a-resource")
	err = dumpResourceDescription(dumpPath, u)

	assert.NoError(t, err)

	dumpPath = path.Join(basePath, resource.GetNamespace(), resource.GetKind(), resource.GetName(), resource.GetName()+".yaml")

	data, err = os.ReadFile(dumpPath)
	assert.NoError(t, err)

	expectedData = "kind: nothing\nmetadata:\n  name: a-resource\n  namespace: ns\n"
	assert.Equal(t, expectedData, string(data))
}
