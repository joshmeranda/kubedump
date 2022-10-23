package tests

import (
	"context"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	apisappsv1 "k8s.io/api/apps/v1"
	apisbatchv1 "k8s.io/api/batch/v1"
	apiscorev1 "k8s.io/api/core/v1"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"kubedump/pkg/controller"
	"kubedump/pkg/filter"
	"kubedump/tests/deployer"
	"os"
	"os/exec"
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

var SampleJob = apisbatchv1.Job{
	ObjectMeta: apismetav1.ObjectMeta{
		Name:      "test-job",
		Namespace: "default",
	},
	Spec: apisbatchv1.JobSpec{
		Template: apiscorev1.PodTemplateSpec{
			ObjectMeta: apismetav1.ObjectMeta{
				Name:      "test-job-pod",
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

// displayTree is a just a utility function to make it easier to debug these tests
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

func setup(t *testing.T) (d deployer.Deployer, client kubernetes.Interface, parentPath string) {
	if found, err := exec.LookPath("kind"); err == nil {
		t.Logf("deploying cluster using 'kind' at '%s'", found)

		if d, err = deployer.NewKindDeployer("", ""); err != nil {
			t.Fatalf("could not create kind deployer: %s", err)
		}
	} else {
		t.Fatal("could not determine suitable k8s deployer")
	}

	if out, err := d.Up(); err != nil {
		t.Fatalf("could not deployer cluster: %s\nOutput:\n%s", err, out)
	}

	config, err := clientcmd.BuildConfigFromFlags("", d.Kubeconfig())

	if err != nil {
		t.Fatalf("could not load config: %s", err)
	}

	client, err = kubernetes.NewForConfig(config)

	if err != nil {
		t.Fatalf("could not crete client: %s", err)
	}

	if dir, err := os.MkdirTemp("", ""); err != nil {
		t.Fatalf("could not create temporary file")
	} else {
		parentPath = path.Join(dir, "kubedump-test")
	}

	for {
		_, err := client.CoreV1().ServiceAccounts("default").Get(context.TODO(), "default", apismetav1.GetOptions{})

		if err == nil {
			break
		}

		t.Log("cluster not yet ready")

		time.Sleep(5 * time.Second)
	}

	t.Log("cluster is ready")

	return
}

func teardown(t *testing.T, d deployer.Deployer, tempDir string) {
	if err := os.RemoveAll(tempDir); err != nil {
		t.Errorf("failed to delete temporary test directory '%s': %s", tempDir, err)
	}

	if err := os.Remove(d.Kubeconfig()); err != nil {
		t.Logf("failed to delete temporary test kubeconfig '%s': %s", d.Kubeconfig(), err)
	}

	if out, err := d.Down(); err != nil {
		t.Logf("failed to delete cluster: %s\nOutput\n%s", err, out)
	}
}

func TestController(t *testing.T) {
	d, client, parentPath := setup(t)
	defer teardown(t, d, parentPath)

	f, _ := filter.Parse("namespace default")

	opts := controller.Options{
		ParentPath: parentPath,
		Filter:     f,
	}

	time.Sleep(5 * time.Second)

	// apply objects to cluster
	_, err := client.CoreV1().Pods("default").Create(context.TODO(), &SamplePod, apismetav1.CreateOptions{})
	defer client.CoreV1().Pods("default").Delete(context.TODO(), SamplePod.Name, deleteOptions())
	if err != nil {
		t.Errorf("could not create pod '%s/%s': %s", SamplePod.Namespace, SamplePod.Name, err)
	}

	_, err = client.BatchV1().Jobs("default").Create(context.TODO(), &SampleJob, apismetav1.CreateOptions{})
	defer client.BatchV1().Jobs("default").Delete(context.TODO(), SampleJob.Name, deleteOptions())
	if err != nil {
		t.Errorf("could not create job '%s/%s': %s", SampleJob.Namespace, SampleJob.Name, err)
	}

	_, err = client.AppsV1().Deployments("default").Create(context.TODO(), &SampleDeployment, apismetav1.CreateOptions{})
	defer client.AppsV1().Deployments("default").Delete(context.TODO(), SampleDeployment.Name, deleteOptions())
	if err != nil {
		t.Errorf("could not create deployment '%s/%s': %s", SampleDeployment.Namespace, SampleDeployment.Name, err)
	}

	c := controller.NewController(client, opts)
	assert.NoError(t, c.Start(5))
	time.Sleep(5 * time.Second)
	assert.NoError(t, c.Stop())

	assertResourceFile(t, "Pod", path.Join(parentPath, SamplePod.Namespace, "pod", SamplePod.Name, SamplePod.Name+".yaml"), SamplePod.GetObjectMeta())
	assertResourceFile(t, "Job", path.Join(parentPath, SampleJob.Namespace, "job", SampleJob.Name, SampleJob.Name+".yaml"), SampleJob.GetObjectMeta())
	assertResourceFile(t, "Deployment", path.Join(parentPath, SampleDeployment.Namespace, "deployment", SampleDeployment.Name, SampleDeployment.Name+".yaml"), SampleDeployment.GetObjectMeta())

	displayTree(t, parentPath)
}
