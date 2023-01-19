package tests

import (
	"context"
	"fmt"
	"github.com/gobwas/glob"
	cp "github.com/otiai10/copy"
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
	"path"
	"sigs.k8s.io/yaml"
	"strings"
	"testing"
	"time"
)

var (
	NWorkers         = 5
	TestWaitDuration = time.Second * 5
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
func CopyTree(target string, destination string) error {
	if err := os.MkdirAll(destination, 0755); err != nil {
		return fmt.Errorf("could not create copy destination '%s': %w", destination, err)
	}

	//if err := exec.Command("cp", "--recursive", target, destination).Run(); err != nil {
	//	return fmt.Errorf("could not copy '%s' -> '%s': %w", target, destination, err)
	//}

	opts := cp.Options{
		OnSymlink: func(string) cp.SymlinkAction {
			return cp.Shallow
		},
	}

	if err := cp.Copy(target, destination, opts); err != nil {
		return fmt.Errorf("could not copy directories: %w", err)
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

func NewHandledResourceNoErr(obj interface{}) kubedump.HandledResource {
	resource, err := kubedump.NewHandledResource(obj)

	if err != nil {
		panic(err.Error())
	}

	return resource
}

func AssertResource(t *testing.T, basePath string, resource kubedump.HandledResource, hasEvents bool) {
	resourceDir := path.Join(basePath, resource.GetNamespace(), resource.Kind, resource.GetName())

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
	case "Secret":
		var secret apicorev1.Secret
		err = unmarshalFile(fileName, &secret)
		fsObj = secret.ObjectMeta
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
	parentPath := path.Join(basePath, parent.GetNamespace(), parent.Kind, parent.GetName())
	linkPath := path.Join(parentPath, linked.Kind, linked.GetName())
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
func createResources(t *testing.T, client kubernetes.Interface, basePath string, ctx context.Context) (func(), func(), error) {
	var aggregatedDefers []func() error
	deferredFunc := func() {
		for _, deferred := range aggregatedDefers {
			if err := deferred(); err != nil {
				t.Errorf("error in deferred function: %s", err)
			}
		}
	}

	var handledResources []kubedump.HandledResource
	waitFunc := func() {
		for _, resource := range handledResources {
			resourcePath := path.Join(basePath, resource.GetNamespace(), resource.Kind, resource.GetName())

			if err := WaitForPath(ctx, TestWaitDuration, resourcePath); err != nil {
				t.Fatalf("failed waiting for resource '%s': %s", resourcePath, err)
			}
		}
	}

	if samplePod, err := client.CoreV1().Pods(ResourceNamespace).Create(ctx, &SamplePod, apimetav1.CreateOptions{}); err != nil {
		return deferredFunc, waitFunc, fmt.Errorf("could not create pod '%s/%s': %s", SamplePod.Namespace, SamplePod.Name, err)
	} else {
		aggregatedDefers = append(aggregatedDefers, func() error {
			return client.CoreV1().Pods(ResourceNamespace).Delete(ctx, SamplePod.Name, DeleteOptions())
		})
		handledResources = append(handledResources, NewHandledResourceNoErr(samplePod))
	}

	if sampleJob, err := client.BatchV1().Jobs(ResourceNamespace).Create(ctx, &SampleJob, apimetav1.CreateOptions{}); err != nil {
		return deferredFunc, waitFunc, fmt.Errorf("could not create job '%s/%s': %s", SampleJob.Namespace, SampleJob.Name, err)
	} else {
		aggregatedDefers = append(aggregatedDefers, func() error {
			return client.BatchV1().Jobs(ResourceNamespace).Delete(ctx, SampleJob.Name, DeleteOptions())
		})
		handledResources = append(handledResources, NewHandledResourceNoErr(sampleJob))
	}

	if sampleReplicaSet, err := client.AppsV1().ReplicaSets(ResourceNamespace).Create(ctx, &SampleReplicaSet, apimetav1.CreateOptions{}); err != nil {
		return deferredFunc, waitFunc, fmt.Errorf("could not create replicaset '%s/%s': %s", SampleReplicaSet.Namespace, SampleReplicaSet.Name, err)
	} else {
		aggregatedDefers = append(aggregatedDefers, func() error {
			return client.AppsV1().ReplicaSets(ResourceNamespace).Delete(ctx, SampleReplicaSet.Name, DeleteOptions())
		})
		handledResources = append(handledResources, NewHandledResourceNoErr(sampleReplicaSet))
	}

	if sampleDeployment, err := client.AppsV1().Deployments(ResourceNamespace).Create(ctx, &SampleDeployment, apimetav1.CreateOptions{}); err != nil {
		return deferredFunc, waitFunc, fmt.Errorf("could not create deployment '%s/%s': %s", SampleDeployment.Namespace, SampleDeployment.Name, err)
	} else {
		aggregatedDefers = append(aggregatedDefers, func() error {
			return client.AppsV1().Deployments(ResourceNamespace).Delete(ctx, SampleDeployment.Name, DeleteOptions())
		})
		handledResources = append(handledResources, NewHandledResourceNoErr(sampleDeployment))
	}

	if sampleServicePod, err := client.CoreV1().Pods(ResourceNamespace).Create(ctx, &SampleServicePod, apimetav1.CreateOptions{}); err != nil {
		return deferredFunc, waitFunc, fmt.Errorf("could not create pod '%s/%s': %s", SampleServicePod.Namespace, SampleServicePod.Name, err)
	} else {
		aggregatedDefers = append(aggregatedDefers, func() error {
			return client.CoreV1().Pods(ResourceNamespace).Delete(ctx, SampleServicePod.Name, DeleteOptions())
		})
		handledResources = append(handledResources, NewHandledResourceNoErr(sampleServicePod))
	}

	if sampleService, err := client.CoreV1().Services(ResourceNamespace).Create(ctx, &SampleService, apimetav1.CreateOptions{}); err != nil {
		return deferredFunc, waitFunc, fmt.Errorf("could not create service '%s/%s': %s", SampleService.Namespace, SampleService.Name, err)
	} else {
		aggregatedDefers = append(aggregatedDefers, func() error {
			return client.CoreV1().Services(ResourceNamespace).Delete(ctx, SampleService.Name, DeleteOptions())
		})
		handledResources = append(handledResources, NewHandledResourceNoErr(sampleService))
	}

	if sampleConfigMap, err := client.CoreV1().ConfigMaps(ResourceNamespace).Create(ctx, &SampleConfigMap, apimetav1.CreateOptions{}); err != nil {
		return deferredFunc, waitFunc, fmt.Errorf("could not create config map '%s/%s': %s", SampleConfigMap.Namespace, SampleConfigMap.Name, err)
	} else {
		aggregatedDefers = append(aggregatedDefers, func() error {
			return client.CoreV1().ConfigMaps(ResourceNamespace).Delete(ctx, SampleConfigMap.Name, DeleteOptions())
		})
		handledResources = append(handledResources, NewHandledResourceNoErr(sampleConfigMap))
	}

	if samplePodWithConfigMapVolume, err := client.CoreV1().Pods(ResourceNamespace).Create(ctx, &SamplePodWithConfigMapVolume, apimetav1.CreateOptions{}); err != nil {
		return deferredFunc, waitFunc, fmt.Errorf("could not create pod '%s/%s': %s", SamplePodWithConfigMapVolume.Namespace, SamplePodWithConfigMapVolume.Name, err)
	} else {
		aggregatedDefers = append(aggregatedDefers, func() error {
			return client.CoreV1().Pods(ResourceNamespace).Delete(ctx, SamplePodWithConfigMapVolume.Name, DeleteOptions())
		})
		handledResources = append(handledResources, NewHandledResourceNoErr(samplePodWithConfigMapVolume))
	}

	if sampleSecret, err := client.CoreV1().Secrets(ResourceNamespace).Create(ctx, &SampleSecret, apimetav1.CreateOptions{}); err != nil {
		return deferredFunc, waitFunc, fmt.Errorf("could not create secret '%s/%s': %s", sampleSecret.Namespace, sampleSecret.Name, err)
	} else {
		aggregatedDefers = append(aggregatedDefers, func() error {
			return client.CoreV1().Secrets(ResourceNamespace).Delete(ctx, SampleSecret.Name, DeleteOptions())
		})
		handledResources = append(handledResources, NewHandledResourceNoErr(sampleSecret))
	}

	if samplePodWithSecretVolume, err := client.CoreV1().Pods(ResourceNamespace).Create(ctx, &SamplePodWithSecretVolume, apimetav1.CreateOptions{}); err != nil {
		return deferredFunc, waitFunc, fmt.Errorf("could not create pod '%s/%s': %s", samplePodWithSecretVolume.Namespace, samplePodWithSecretVolume.Name, err)
	} else {
		aggregatedDefers = append(aggregatedDefers, func() error {
			return client.CoreV1().Secrets(ResourceNamespace).Delete(ctx, SamplePodWithSecretVolume.Name, DeleteOptions())
		})
		handledResources = append(handledResources, NewHandledResourceNoErr(samplePodWithSecretVolume))
	}

	return deferredFunc, waitFunc, nil
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
