package tests

import (
	"context"
	"fmt"
	"github.com/gobwas/glob"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	apiappsv1 "k8s.io/api/apps/v1"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	kubedump "kubedump/pkg"
	"os"
	"os/exec"
	"path"
	"sigs.k8s.io/yaml"
	"strings"
	"testing"
	"time"
)

// isSymlink determines whether the given file path points to a symlink.
func isSymlink(filePath string) (bool, error) {
	info, err := os.Lstat(filePath)

	if err != nil {
		return false, fmt.Errorf("could not stat file '%s': %s", filePath, err)
	}

	return info.Mode()&os.ModeSymlink == os.ModeSymlink, nil
}

// CopyTree will copy the target directory to the destination directory.
//
// this isn't an ideal implementation by any means, but it's simpler than doing it via golang.
func CopyTree(target string, destination string) error {
	if err := os.MkdirAll(destination, 0755); err != nil {
		return fmt.Errorf("could not create copy destination '%s': %w", destination, err)
	}

	if err := exec.Command("cp", "--recursive", target, destination).Run(); err != nil {
		return fmt.Errorf("could not copy '%s' -> '%s': %w", target, destination, err)
	}

	return nil
}

// unmarshalFile will attempt to marshal teh file at the given path into the given object.
func unmarshalFile(fileName string, obj interface{}) error {
	data, err := ioutil.ReadFile(fileName)

	if err != nil {
		return fmt.Errorf("could not unmarshal file '%s': %w", fileName, err)
	}

	err = yaml.Unmarshal(data, obj)

	return err
}

// findGlobsIn will find all top level files in the given directory that match the given pattern.
func findGlobsIn(parent string, pattern glob.Glob) ([]string, error) {
	var found []string

	children, err := os.ReadDir(parent)
	if err != nil {
		return nil, fmt.Errorf("could not read directory '%s': %w", parent, err)
	}

	for _, child := range children {
		if pattern.Match(child.Name()) {
			found = append(found, path.Join(parent, child.Name()))
		}
	}

	return found, nil
}

func newHandledResourceNoErr(obj interface{}) kubedump.HandledResource {
	resource, err := kubedump.NewHandledResource("", obj)

	if err != nil {
		panic(err.Error())
	}

	return resource
}

func AssertResource(t *testing.T, basePath string, resource kubedump.HandledResource, hasEvents bool) {
	resourceDir := path.Join(basePath, resource.GetNamespace(), strings.ToLower(resource.Kind), resource.GetName())

	assertResourceFile(t, resource.Kind, path.Join(resourceDir, resource.GetName()+".yaml"), resource)

	if hasEvents {
		assert.FileExists(t, path.Join(resourceDir, resource.GetName()+".events"))
	}
}

// assertResourceFile will assert if the expected given object matches the file as stored to the filesystem.
func assertResourceFile(t *testing.T, kind string, fileName string, obj apimetav1.Object) {
	var fsObj apimetav1.ObjectMeta
	var err error

	switch kind {
	case "Pod":
		var pod apicorev1.Pod
		err = unmarshalFile(fileName, &pod)
		fsObj = pod.ObjectMeta
	case "Job":
		var job apibatchv1.Job
		err = unmarshalFile(fileName, &job)
		fsObj = job.ObjectMeta
	case "ReplicaSet":
		var set apiappsv1.ReplicaSet
		err = unmarshalFile(fileName, &set)
		fsObj = set.ObjectMeta
	case "Deployment":
		var deployment apiappsv1.Deployment
		err = unmarshalFile(fileName, &deployment)
		fsObj = deployment.ObjectMeta
	case "Service":
		var service apicorev1.Service
		err = unmarshalFile(fileName, &service)
		fsObj = service.ObjectMeta
	case "ConfigMap":
		var configmap apicorev1.ConfigMap
		err = unmarshalFile(fileName, &configmap)
		fsObj = configmap.ObjectMeta
	default:
		t.Errorf("unsupported kind '%s' encountered", kind)
	}

	assert.NoError(t, err)

	assert.Equal(t, obj.GetName(), fsObj.GetName())
	assert.Equal(t, obj.GetNamespace(), fsObj.GetNamespace())
}

func AssertLinkGlob(t *testing.T, parent string, pattern glob.Glob) {
	found, err := findGlobsIn(parent, pattern)

	assert.NoError(t, err)
	assert.Equal(t, 1, len(found))

	for _, p := range found {
		if err != nil {
			assert.NoError(t, err)
		} else {
			isLink, err := isSymlink(p)
			assert.NoError(t, err)
			assert.Truef(t, isLink, "file '%s' should be a symlink", p)
		}
	}
}

func AssertResourceIsLinked(t *testing.T, basePath string, parent kubedump.HandledResource, linked kubedump.HandledResource) {
	parentPath := path.Join(basePath, parent.GetNamespace(), strings.ToLower(parent.Kind), parent.GetName())
	linkPath := path.Join(parentPath, strings.ToLower(linked.Kind), linked.GetName())
	isLink, err := isSymlink(linkPath)

	assert.NoError(t, err)
	assert.True(t, isLink)
}

const (
	ResourceNamespace = "default"

	kubedumpTestLabelKey   = "kubedump-test"
	kubedumpTestLabelValue = ""
)

func DeleteOptions() apimetav1.DeleteOptions {
	policy := apimetav1.DeletePropagationBackground

	return apimetav1.DeleteOptions{
		PropagationPolicy: &policy,
	}
}

// createResources creates one of each target resource and returns a function that can be called to delete one of each
func createResources(t *testing.T, client kubernetes.Interface) (func(), error) {
	var aggregatedDefers []func() error
	deferredFunc := func() {
		for _, deferred := range aggregatedDefers {
			if err := deferred(); err != nil {
				t.Errorf("error in deferred function: %s", err)
			}
		}
	}

	_, err := client.CoreV1().Pods(ResourceNamespace).Create(context.Background(), &SamplePod, apimetav1.CreateOptions{})
	aggregatedDefers = append(aggregatedDefers, func() error {
		return client.CoreV1().Pods(ResourceNamespace).Delete(context.Background(), SamplePod.Name, DeleteOptions())
	})
	if err != nil {
		return deferredFunc, fmt.Errorf("could not create pod '%s/%s': %s", SamplePod.Namespace, SamplePod.Name, err)
	}

	_, err = client.BatchV1().Jobs(ResourceNamespace).Create(context.Background(), &SampleJob, apimetav1.CreateOptions{})
	aggregatedDefers = append(aggregatedDefers, func() error {
		return client.BatchV1().Jobs(ResourceNamespace).Delete(context.Background(), SampleJob.Name, DeleteOptions())
	})
	if err != nil {
		return deferredFunc, fmt.Errorf("could not create job '%s/%s': %s", SampleJob.Namespace, SampleJob.Name, err)
	}

	_, err = client.AppsV1().ReplicaSets(ResourceNamespace).Create(context.Background(), &SampleReplicaSet, apimetav1.CreateOptions{})
	aggregatedDefers = append(aggregatedDefers, func() error {
		return client.AppsV1().ReplicaSets(ResourceNamespace).Delete(context.Background(), SampleReplicaSet.Name, DeleteOptions())
	})
	if err != nil {
		return deferredFunc, fmt.Errorf("could not create replicaset '%s/%s': %s", SampleReplicaSet.Namespace, SampleReplicaSet.Name, err)
	}

	_, err = client.AppsV1().Deployments(ResourceNamespace).Create(context.Background(), &SampleDeployment, apimetav1.CreateOptions{})
	aggregatedDefers = append(aggregatedDefers, func() error {
		return client.AppsV1().Deployments(ResourceNamespace).Delete(context.Background(), SampleDeployment.Name, DeleteOptions())
	})
	if err != nil {
		return deferredFunc, fmt.Errorf("could not create deployment '%s/%s': %s", SampleDeployment.Namespace, SampleDeployment.Name, err)
	}

	_, err = client.CoreV1().Pods(ResourceNamespace).Create(context.Background(), &SampleServicePod, apimetav1.CreateOptions{})
	aggregatedDefers = append(aggregatedDefers, func() error {
		return client.CoreV1().Pods(ResourceNamespace).Delete(context.Background(), SampleServicePod.Name, DeleteOptions())
	})
	if err != nil {
		return deferredFunc, fmt.Errorf("could not create pod '%s/%s': %s", SampleServicePod.Namespace, SampleServicePod.Name, err)
	}

	_, err = client.CoreV1().Services(ResourceNamespace).Create(context.Background(), &SampleService, apimetav1.CreateOptions{})
	aggregatedDefers = append(aggregatedDefers, func() error {
		return client.CoreV1().Services(ResourceNamespace).Delete(context.Background(), SampleService.Name, DeleteOptions())
	})
	if err != nil {
		return deferredFunc, fmt.Errorf("could not create service '%s/%s': %s", SampleService.Namespace, SampleService.Name, err)
	}

	_, err = client.CoreV1().ConfigMaps(ResourceNamespace).Create(context.Background(), &SampleConfigMap, apimetav1.CreateOptions{})
	aggregatedDefers = append(aggregatedDefers, func() error {
		return client.CoreV1().ConfigMaps(ResourceNamespace).Delete(context.Background(), SampleConfigMap.Name, DeleteOptions())
	})
	if err != nil {
		return deferredFunc, fmt.Errorf("could not create config map '%s/%s': %s", SampleConfigMap.Namespace, SampleConfigMap.Name, err)
	}

	_, err = client.CoreV1().Pods(ResourceNamespace).Create(context.Background(), &SamplePodWithConfigMapVolume, apimetav1.CreateOptions{})
	aggregatedDefers = append(aggregatedDefers, func() error {
		return client.CoreV1().Pods(ResourceNamespace).Delete(context.Background(), SamplePodWithConfigMapVolume.Name, DeleteOptions())
	})
	if err != nil {
		return deferredFunc, fmt.Errorf("could not create pod '%s/%s': %s", SamplePodWithConfigMapVolume.Namespace, SamplePodWithConfigMapVolume.Name, err)
	}

	return deferredFunc, nil
}

// WaitForPath will block until a file at the given path exists.
func WaitForPath(parentContext context.Context, timeout time.Duration, path string) error {
	ctx, cancel := context.WithTimeout(parentContext, timeout)

	wait.UntilWithContext(ctx, func(ctx context.Context) {
		_, err := os.Stat(path)

		if !os.IsNotExist(err) {
			cancel()
		}
	}, time.Second)

	if err := ctx.Err(); err != nil && !strings.Contains(err.Error(), "context canceled") {
		return err
	}

	return nil
}

func WaitForResources(t *testing.T, basePath string, ctx context.Context, handledResources ...kubedump.HandledResource) {
	for _, resource := range handledResources {
		// todo: this will break for non-namespaced (might want to expose pkg/controller/utils/resourceDirPath)
		dir := path.Join(basePath, resource.GetNamespace(), strings.ToLower(resource.Kind), resource.GetName())

		if err := WaitForPath(ctx, time.Second*5, dir); err != nil {
			t.Errorf("error waiting for resource dir '%s': %s", resource, err)
		}
	}
}
