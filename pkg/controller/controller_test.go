package controller

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	apibatchv1 "k8s.io/api/batch/v1"
	apiscorev1 "k8s.io/api/core/v1"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	testing2 "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/clientcmd"
	"kubedump/pkg/filter"
	"os"
	"path"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"testing"
	"time"
)

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

var SampleJob = apibatchv1.Job{
	ObjectMeta: apismetav1.ObjectMeta{
		Name:      "test-job",
		Namespace: "default",
	},
	Spec: apibatchv1.JobSpec{
		Parallelism:           nil,
		Completions:           nil,
		ActiveDeadlineSeconds: nil,
		BackoffLimit:          nil,
		Selector:              nil,
		ManualSelector:        nil,
		Template: apiscorev1.PodTemplateSpec{
			ObjectMeta: apismetav1.ObjectMeta{
				Name:      "test-job-pod",
				Namespace: "default",
			},
			Spec: SamplePodSpec,
		},
		TTLSecondsAfterFinished: nil,
		CompletionMode:          nil,
		Suspend:                 nil,
	},
}

// displayTree is a just a utility function to make it easier to debug these tests
func displayTree(t *testing.T, dir string) {
	err := filepath.Walk(dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			fmt.Println(path)
			return nil
		})

	if err != nil {
		t.Logf("error walking directory '%s': %s", dir, err)
	}
}

func deleteOptions() apismetav1.DeleteOptions {
	policy := apismetav1.DeletePropagationBackground

	return apismetav1.DeleteOptions{
		PropagationPolicy: &policy,
	}
}

func unmarshalFile(fileName string, obj interface{}) error {
	data, err := ioutil.ReadFile(fileName)

	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, obj)

	return err
}

func assertResourceFile(t *testing.T, kind string, fileName string, obj apismetav1.Object) {
	var fsObj apismetav1.ObjectMeta
	var err error

	switch kind {
	case "Pod":
		var pod apiscorev1.Pod
		err = unmarshalFile(fileName, &pod)
		fsObj = pod.ObjectMeta
	case "Job":
		var job apibatchv1.Job
		err = unmarshalFile(fileName, &job)
		fsObj = job.ObjectMeta
	default:
		t.Errorf("unsupported kind '%s' encountered", kind)
	}

	assert.NoError(t, err)

	assert.Equal(t, obj.GetName(), fsObj.GetName())
	assert.Equal(t, obj.GetNamespace(), fsObj.GetNamespace())
}

func setup(t *testing.T, fakeClient bool) (client kubernetes.Interface, parentPath string) {
	if fakeClient {
		watcherStarted := make(chan struct{})

		fakeClient := fake.NewSimpleClientset()
		fakeClient.PrependWatchReactor("*", func(action testing2.Action) (bool, watch.Interface, error) {
			gvy := action.GetResource()
			ns := action.GetNamespace()
			watcher, err := fakeClient.Tracker().Watch(gvy, ns)

			if err != nil {
				t.Fatalf("error setting watch reactor")
			}

			close(watcherStarted)

			return true, watcher, nil
		})
		<-watcherStarted

		client = fakeClient

	} else {
		config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))

		if err != nil {
			t.Fatalf("could not load config: %s", err)
		}

		client, err = kubernetes.NewForConfig(config)

		if err != nil {
			t.Fatalf("could not crete client: %s", err)
		}
	}

	if dir, err := os.MkdirTemp("", ""); err != nil {
		t.Fatalf("could not create temporary file")
	} else {
		parentPath = path.Join(dir, "kubedump-test")
	}

	return
}

func teardown(t *testing.T, tempDir string) {
	if err := os.RemoveAll(tempDir); err != nil {
		t.Logf("[WARNING] failed to delete temporary test directory '%s': %s", tempDir, err)
	}
}

func TestController(t *testing.T) {
	client, parentPath := setup(t, false)
	defer teardown(t, parentPath)

	f, _ := filter.Parse("namespace default")

	opts := Options{
		ParentPath: parentPath,
		Filter:     f,
	}

	// apply objects to cluster
	_, err := client.CoreV1().Pods("default").Create(context.TODO(), &SamplePod, apismetav1.CreateOptions{})
	defer client.CoreV1().Pods("default").Delete(context.TODO(), SamplePod.Name, deleteOptions())
	assert.NoError(t, err)

	_, err = client.BatchV1().Jobs("default").Create(context.TODO(), &SampleJob, apismetav1.CreateOptions{})
	defer client.BatchV1().Jobs("default").Delete(context.TODO(), SampleJob.Name, deleteOptions())

	c := NewController(client, opts)
	assert.NoError(t, c.Start(5))
	time.Sleep(5 * time.Second)
	assert.NoError(t, c.Stop())

	//displayTree(t, parentPath)

	assertResourceFile(t, "Pod", path.Join(parentPath, SamplePod.Namespace, "pod", SamplePod.Name, SamplePod.Name+".yaml"), SamplePod.GetObjectMeta())
	assertResourceFile(t, "Job", path.Join(parentPath, SampleJob.Namespace, "job", SampleJob.Name, SampleJob.Name+".yaml"), SampleJob.GetObjectMeta())
}
