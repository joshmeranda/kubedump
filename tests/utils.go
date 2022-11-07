package tests

import (
	"context"
	"fmt"
	"github.com/gobwas/glob"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	apisappsv1 "k8s.io/api/apps/v1"
	apisbatchv1 "k8s.io/api/batch/v1"
	apiscorev1 "k8s.io/api/core/v1"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"os"
	"path"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"strings"
	"testing"
)

// exists checks if a file exists.
func exists(filePath string) bool {
	_, err := os.Lstat(filePath)

	return !os.IsNotExist(err)
}

// displayTree will display the entire directory structure pointed to by dir.
func displayTree(t *testing.T, dir string) {
	t.Log()
	err := filepath.Walk(dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			t.Log(path)
			return nil
		})
	t.Log()

	if err != nil {
		t.Logf("error walking directory '%s': %s", dir, err)
	}
}

// copyTree will copy the target directory to the destination directory.
//
// this is not necessarily the most optimized implementation but for a testing util it'll do.
func copyTree(t *testing.T, target string, destination string) {
	target, _ = filepath.Abs(target)
	destination, _ = filepath.Abs(destination)

	if exists(destination) {
		t.Errorf("directory '%s' already exists, not copying", destination)
		return
	}

	err := filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
		newPath := filepath.Join(destination, strings.TrimPrefix(path, target))

		if info.IsDir() {
			if err = os.MkdirAll(newPath, info.Mode()); err != nil {
				t.Errorf("could not copy directory '%s': %e", path, err)
			}
		} else {
			data, err := ioutil.ReadFile(path)

			if err != nil {
				t.Errorf("could not copy file '%s': %s", path, err)
			}

			if err := ioutil.WriteFile(newPath, data, info.Mode()); err != nil {
				t.Errorf("could not write to file '%s': %s", newPath, err)
			}
		}

		return nil
	})

	if err != nil {
		t.Logf("error walking directory")
	}
}

// unmarshalFile will attempt to marshal teh file at the given path into the given object.
func unmarshalFile(fileName string, obj interface{}) error {
	data, err := ioutil.ReadFile(fileName)

	if err != nil {
		return err
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

// assertResourceFile will assert if the expected iven object matches the file as stored to the filesystem.
func assertResourceFile(t *testing.T, kind string, fileName string, obj apismetav1.Object) {
	var fsObj apismetav1.ObjectMeta
	var err error

	switch kind {
	case "Pod":
		var pod apiscorev1.Pod
		err = unmarshalFile(fileName, &pod)
		fsObj = pod.ObjectMeta
	case "Job":
		var job apisbatchv1.Job
		err = unmarshalFile(fileName, &job)
		fsObj = job.ObjectMeta
	case "Deployment":
		var deployment apisappsv1.Deployment
		err = unmarshalFile(fileName, &deployment)
		fsObj = deployment.ObjectMeta
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
		stat, err := os.Lstat(p)

		if err != nil {
			assert.NoError(t, err)
		} else {
			assert.Equal(t, os.ModeSymlink, stat.Mode()&os.ModeSymlink)
		}
	}
}

var SamplePodSpec = apiscorev1.PodSpec{
	Containers: []apiscorev1.Container{
		{
			Name:            "test-container",
			Image:           "alpine:latest",
			Command:         []string{"sh", "-c", "while :; do date '+%F %T %z'; sleep 5; done"},
			ImagePullPolicy: "",
		},
	},
	RestartPolicy: "Never",
}

var SamplePod = apiscorev1.Pod{
	ObjectMeta: apismetav1.ObjectMeta{
		Name:      "test-pod",
		Namespace: "default",
	},
	Spec: SamplePodSpec,
}

var SampleJob = apisbatchv1.Job{
	ObjectMeta: apismetav1.ObjectMeta{
		Name:      "test-job",
		Namespace: "default",
	},
	Spec: apisbatchv1.JobSpec{
		Template: apiscorev1.PodTemplateSpec{
			ObjectMeta: apismetav1.ObjectMeta{
				Namespace: "default",
			},
			Spec: SamplePodSpec,
		},
	},
}

var SampleDeployment = apisappsv1.Deployment{
	ObjectMeta: apismetav1.ObjectMeta{
		Name:      "test-deployment",
		Namespace: "default",
		Labels: map[string]string{
			"test-label-key": "test-label-value",
		},
	},
	Spec: apisappsv1.DeploymentSpec{
		Selector: &apismetav1.LabelSelector{
			MatchLabels: map[string]string{
				"test-label-key": "test-label-value",
			},
			MatchExpressions: nil,
		},
		Template: apiscorev1.PodTemplateSpec{
			ObjectMeta: apismetav1.ObjectMeta{
				Name:      "test-job-pod",
				Namespace: "default",
				Labels: map[string]string{
					"test-label-key": "test-label-value",
				},
			},
			Spec: apiscorev1.PodSpec{
				Containers: []apiscorev1.Container{
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

func deleteOptions() apismetav1.DeleteOptions {
	policy := apismetav1.DeletePropagationBackground

	return apismetav1.DeleteOptions{
		PropagationPolicy: &policy,
	}
}

// createResources creates one of each target resource and returns a function that can be called to delete one of each
func createResources(t *testing.T, client kubernetes.Interface) (func(), error) {
	var aggregatedDefers []func() error

	_, err := client.CoreV1().Pods("default").Create(context.TODO(), &SamplePod, apismetav1.CreateOptions{})
	aggregatedDefers = append(aggregatedDefers, func() error {
		return client.CoreV1().Pods("default").Delete(context.TODO(), SamplePod.Name, deleteOptions())
	})
	if err != nil {
		t.Errorf("could not create pod '%s/%s': %s", SamplePod.Namespace, SamplePod.Name, err)
	}

	_, err = client.BatchV1().Jobs("default").Create(context.TODO(), &SampleJob, apismetav1.CreateOptions{})
	aggregatedDefers = append(aggregatedDefers, func() error {
		return client.BatchV1().Jobs("default").Delete(context.TODO(), SampleJob.Name, deleteOptions())
	})
	if err != nil {
		t.Errorf("could not create job '%s/%s': %s", SampleJob.Namespace, SampleJob.Name, err)
	}

	_, err = client.AppsV1().Deployments("default").Create(context.TODO(), &SampleDeployment, apismetav1.CreateOptions{})
	aggregatedDefers = append(aggregatedDefers, func() error {
		return client.AppsV1().Deployments("default").Delete(context.TODO(), SampleDeployment.Name, deleteOptions())
	})
	if err != nil {
		t.Errorf("could not create deployment '%s/%s': %s", SampleDeployment.Namespace, SampleDeployment.Name, err)
	}

	return func() {
		for _, deferred := range aggregatedDefers {
			if err := deferred(); err != nil {
				t.Errorf("error in deferred function: %s", err)
			}
		}
	}, nil
}
