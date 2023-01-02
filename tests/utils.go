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
	"k8s.io/client-go/kubernetes"
	kubedump "kubedump/pkg"
	"os"
	"os/exec"
	"path"
	"sigs.k8s.io/yaml"
	"strings"
	"testing"
)

// isSymlink determines whether the given file path points to a symlink.
func isSymlink(filePath string) (bool, error) {
	info, err := os.Lstat(filePath)

	if err != nil {
		return false, fmt.Errorf("could not stat file '%s': %s", filePath, err)
	}

	return info.Mode()&os.ModeSymlink == os.ModeSymlink, nil
}

// copyTree will copy the target directory to the destination directory.
//
// this isn't an ideal implementation by any means at all, but it's simpler than doing  it all manually.
func copyTree(t *testing.T, target string, destination string) {
	if err := exec.Command("cp", "--recursive", target, destination).Run(); err != nil {
		t.Errorf("could not copy '%s' -> '%s': %s", target, destination, err)
	}
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

func assertResource(t *testing.T, basePath string, resource kubedump.HandledResource, hasEvents bool) {
	resourceDir := path.Join(basePath, resource.GetNamespace(), strings.ToLower(resource.Kind), resource.GetName())

	assertResourceFile(t, resource.Kind, path.Join(resourceDir, resource.GetName()+".yaml"), resource)

	if hasEvents {
		assert.FileExists(t, path.Join(resourceDir, resource.GetName()+".events"))
	}
}

// assertResourceFile will assert if the expected iven object matches the file as stored to the filesystem.
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

func assertLinkGlob(t *testing.T, parent string, pattern glob.Glob) {
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

const (
	ResourceNamespace = "default"

	kubedumpTestLabelKey   = "kubedump-test"
	kubedumpTestLabelValue = ""
)

var SamplePodSpec = apicorev1.PodSpec{
	Containers: []apicorev1.Container{
		{
			Name:            "test-container",
			Image:           "alpine:latest",
			Command:         []string{"sh", "-c", "while :; do date '+%F %T %z'; sleep 1; done"},
			ImagePullPolicy: "",
		},
	},
	RestartPolicy: "Never",
}

var SamplePod = apicorev1.Pod{
	ObjectMeta: apimetav1.ObjectMeta{
		Name:      "test-pod",
		Namespace: ResourceNamespace,
		Labels: map[string]string{
			kubedumpTestLabelKey: kubedumpTestLabelValue,
		},
	},
	Spec: SamplePodSpec,
}

var SampleJob = apibatchv1.Job{
	ObjectMeta: apimetav1.ObjectMeta{
		Name:      "test-job",
		Namespace: ResourceNamespace,
		Labels: map[string]string{
			kubedumpTestLabelKey: kubedumpTestLabelValue,
		},
	},
	Spec: apibatchv1.JobSpec{
		Template: apicorev1.PodTemplateSpec{
			ObjectMeta: apimetav1.ObjectMeta{
				Namespace: ResourceNamespace,
			},
			Spec: SamplePodSpec,
		},
	},
}

var SampleReplicaSet = apiappsv1.ReplicaSet{
	ObjectMeta: apimetav1.ObjectMeta{
		Name:      "test-replicaset",
		Namespace: ResourceNamespace,
		Labels: map[string]string{
			kubedumpTestLabelKey: kubedumpTestLabelValue,
		},
	},
	Spec: apiappsv1.ReplicaSetSpec{
		Selector: &apimetav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "test-replicaset",
			},
			MatchExpressions: nil,
		},
		Template: apicorev1.PodTemplateSpec{
			ObjectMeta: apimetav1.ObjectMeta{
				Namespace: ResourceNamespace,
				Labels: map[string]string{
					"app": "test-replicaset",
				},
			},
			Spec: apicorev1.PodSpec{
				Containers: []apicorev1.Container{
					{
						Name:            "test-container",
						Image:           "alpine:latest",
						Command:         []string{"sh", "-c", "while :; do date '+%F %T %z'; sleep 5; done"},
						ImagePullPolicy: "",
					},
				},
			},
		},
	},
}

var SampleDeployment = apiappsv1.Deployment{
	ObjectMeta: apimetav1.ObjectMeta{
		Name:      "test-deployment",
		Namespace: ResourceNamespace,
		Labels: map[string]string{
			"app": "test-deployment",
		},
	},
	Spec: apiappsv1.DeploymentSpec{
		Selector: &apimetav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "test-deployment",
			},
			MatchExpressions: nil,
		},
		Template: apicorev1.PodTemplateSpec{
			ObjectMeta: apimetav1.ObjectMeta{
				Namespace: ResourceNamespace,
				Labels: map[string]string{
					"app": "test-deployment",
				},
			},
			Spec: apicorev1.PodSpec{
				Containers: []apicorev1.Container{
					{
						Name:            "test-container",
						Image:           "alpine:latest",
						Command:         []string{"sh", "-c", "while :; do date '+%F %T %z'; sleep 5; done"},
						ImagePullPolicy: "",
					},
				},
			},
		},
	},
}

var SampleServicePod = apicorev1.Pod{
	ObjectMeta: apimetav1.ObjectMeta{
		Name:      "test-service-pod",
		Namespace: ResourceNamespace,
		Labels: map[string]string{
			"app": "test-service",
		},
	},
	Spec: SamplePodSpec,
}

var SampleService = apicorev1.Service{
	ObjectMeta: apimetav1.ObjectMeta{
		Name:      "test-service",
		Namespace: ResourceNamespace,
		Labels: map[string]string{
			"app":                "test-service",
			kubedumpTestLabelKey: kubedumpTestLabelValue,
		},
	},
	Spec: apicorev1.ServiceSpec{
		Ports: []apicorev1.ServicePort{
			{
				Protocol: "TCP",
				Port:     80,
			},
		},
		Selector: map[string]string{
			"app": "test-service",
		},
	},
}

var SampleConfigMap = apicorev1.ConfigMap{
	ObjectMeta: apimetav1.ObjectMeta{
		Name:      "test-configmap",
		Namespace: ResourceNamespace,
	},
	Immutable: nil,
	Data: map[string]string{
		"a": "A",
		"b": "B",
		"c": "C",
	},
	BinaryData: nil,
}

func deleteOptions() apimetav1.DeleteOptions {
	policy := apimetav1.DeletePropagationBackground

	return apimetav1.DeleteOptions{
		PropagationPolicy: &policy,
	}
}

// createResources creates one of each target resource and returns a function that can be called to delete one of each
func createResources(t *testing.T, client kubernetes.Interface) (func(), error) {
	var aggregatedDefers []func() error

	_, err := client.CoreV1().Pods(ResourceNamespace).Create(context.TODO(), &SamplePod, apimetav1.CreateOptions{})
	aggregatedDefers = append(aggregatedDefers, func() error {
		return client.CoreV1().Pods(ResourceNamespace).Delete(context.TODO(), SamplePod.Name, deleteOptions())
	})
	if err != nil {
		t.Errorf("could not create pod '%s/%s': %s", SamplePod.Namespace, SamplePod.Name, err)
	}

	_, err = client.BatchV1().Jobs(ResourceNamespace).Create(context.TODO(), &SampleJob, apimetav1.CreateOptions{})
	aggregatedDefers = append(aggregatedDefers, func() error {
		return client.BatchV1().Jobs(ResourceNamespace).Delete(context.TODO(), SampleJob.Name, deleteOptions())
	})
	if err != nil {
		t.Errorf("could not create job '%s/%s': %s", SampleJob.Namespace, SampleJob.Name, err)
	}

	_, err = client.AppsV1().ReplicaSets(ResourceNamespace).Create(context.TODO(), &SampleReplicaSet, apimetav1.CreateOptions{})
	aggregatedDefers = append(aggregatedDefers, func() error {
		return client.AppsV1().ReplicaSets(ResourceNamespace).Delete(context.TODO(), SampleReplicaSet.Name, deleteOptions())
	})
	if err != nil {
		t.Errorf("could not create replicaset '%s/%s': %s", SampleReplicaSet.Namespace, SampleReplicaSet.Name, err)
	}

	_, err = client.AppsV1().Deployments(ResourceNamespace).Create(context.TODO(), &SampleDeployment, apimetav1.CreateOptions{})
	aggregatedDefers = append(aggregatedDefers, func() error {
		return client.AppsV1().Deployments(ResourceNamespace).Delete(context.TODO(), SampleDeployment.Name, deleteOptions())
	})
	if err != nil {
		t.Errorf("could not create deployment '%s/%s': %s", SampleDeployment.Namespace, SampleDeployment.Name, err)
	}

	_, err = client.CoreV1().Pods(ResourceNamespace).Create(context.TODO(), &SampleServicePod, apimetav1.CreateOptions{})
	aggregatedDefers = append(aggregatedDefers, func() error {
		return client.CoreV1().Pods(ResourceNamespace).Delete(context.TODO(), SampleServicePod.Name, deleteOptions())
	})
	if err != nil {
		t.Errorf("could not create pod '%s/%s': %s", SampleServicePod.Namespace, SampleServicePod.Name, err)
	}

	_, err = client.CoreV1().Services(ResourceNamespace).Create(context.TODO(), &SampleService, apimetav1.CreateOptions{})
	aggregatedDefers = append(aggregatedDefers, func() error {
		return client.CoreV1().Services(ResourceNamespace).Delete(context.TODO(), SampleService.Name, deleteOptions())
	})
	if err != nil {
		t.Errorf("could not create service '%s/%s': %s", SampleService.Namespace, SampleService.Name, err)
	}

	_, err = client.CoreV1().ConfigMaps(ResourceNamespace).Create(context.TODO(), &SampleConfigMap, apimetav1.CreateOptions{})
	aggregatedDefers = append(aggregatedDefers, func() error {
		return client.CoreV1().ConfigMaps(ResourceNamespace).Delete(context.TODO(), SampleConfigMap.Name, deleteOptions())
	})
	if err != nil {
		t.Errorf("could not create config map '%s/%s': %s", SampleConfigMap.Namespace, SampleConfigMap.Name, err)
	}

	return func() {
		for _, deferred := range aggregatedDefers {
			if err := deferred(); err != nil {
				t.Errorf("error in deferred function: %s", err)
			}
		}
	}, nil
}
