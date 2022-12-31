package tests

import (
	"context"
	"fmt"
	"github.com/gobwas/glob"
	"github.com/stretchr/testify/assert"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	kubedump "kubedump/pkg/cmd"
	"kubedump/tests/deployer"
	"os"
	"os/exec"
	"path"
	"testing"
	"time"
)

func controllerSetup(t *testing.T) (d deployer.Deployer, client kubernetes.Interface, parentPath string) {
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

	stopChan := make(chan struct{})
	wait.Until(func() {
		_, err := client.CoreV1().ServiceAccounts("default").Get(context.TODO(), "default", apimetav1.GetOptions{})

		if err == nil {
			close(stopChan)
			return
		}

		t.Log("cluster not yet ready")

	}, time.Second*5, stopChan)

	t.Log("cluster is ready")

	return
}

func controllerTeardown(t *testing.T, d deployer.Deployer, tempDir string) {
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

func checkPods(t *testing.T, client kubernetes.Interface, stopCh chan struct{}) {
	list, err := client.CoreV1().Pods("default").List(context.Background(), apimetav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", kubedumpTestLabelKey, kubedumpTestLabelValue),
	})

	if err != nil {
		t.Errorf("failed to list pods: %s", err)
		close(stopCh)
	}

	for _, pod := range list.Items {
		if pod.Status.Phase != apicorev1.PodRunning {
			t.Logf("pod '%s/%s' is not running", pod.Namespace, pod.Name)
			continue
		}

		for _, container := range pod.Status.ContainerStatuses {
			if container.State.Running == nil {
				t.Logf("container '%s' in pod '%s/%s' is not runnig", container.Name, pod.Namespace, pod.Name)
			}
		}
	}

	close(stopCh)
}

func TestDump(t *testing.T) {
	d, client, parentPath := controllerSetup(t)
	defer controllerTeardown(t, d, parentPath)

	deferred, err := createResources(t, client)
	assert.NoError(t, err)
	defer deferred()

	// block until pods are running
	stopCh := make(chan struct{})
	wait.Until(func() { checkPods(t, client, stopCh) }, time.Second*5, stopCh)
	<-stopCh

	stopChan := make(chan interface{})
	done := make(chan interface{})

	go func() {
		verbose := false
		nWorkers := fmt.Sprintf("%d", 10)

		app := kubedump.NewKubedumpApp(stopChan)

		var err error
		if verbose {
			err = app.Run([]string{"kubedump", "--kubeconfig", d.Kubeconfig(), "dump", "--verbose", "--workers", nWorkers, "--destination", parentPath, "--filter", "namespace default"})
		} else {
			err = app.Run([]string{"kubedump", "--kubeconfig", d.Kubeconfig(), "dump", "--workers", nWorkers, "--destination", parentPath, "--filter", "namespace default"})
		}

		assert.NoError(t, err)

		close(done)

		t.Log("kubedump is finished")
	}()

	time.Sleep(30 * time.Second)

	close(stopChan)
	<-done

	//displayTree(t, parentPath)
	//copyTree(t, parentPath, d.Name()+".dump")

	assertResourceFile(t, "Pod", path.Join(parentPath, SamplePod.Namespace, "pod", SamplePod.Name, SamplePod.Name+".yaml"), SamplePod.GetObjectMeta())

	assertResourceFile(t, "Job", path.Join(parentPath, SampleJob.Namespace, "job", SampleJob.Name, SampleJob.Name+".yaml"), SampleJob.GetObjectMeta())
	assertLinkGlob(t, path.Join(parentPath, SampleJob.Namespace, "job", SampleJob.Name, "pod"), glob.MustCompile(fmt.Sprintf("%s-*", SampleJob.Name)))

	assertResourceFile(t, "ReplicaSet", path.Join(parentPath, SampleReplicaSet.Namespace, "replicaset", SampleReplicaSet.Name, SampleReplicaSet.Name+".yaml"), SampleReplicaSet.GetObjectMeta())

	assertResourceFile(t, "Deployment", path.Join(parentPath, SampleDeployment.Namespace, "deployment", SampleDeployment.Name, SampleDeployment.Name+".yaml"), SampleDeployment.GetObjectMeta())
	assertLinkGlob(t, path.Join(parentPath, SampleDeployment.Namespace, "deployment", SampleDeployment.Name, "replicaset"), glob.MustCompile(fmt.Sprintf("%s-*", SampleDeployment.Name)))

	assertResourceFile(t, "Service", path.Join(parentPath, SampleService.Namespace, "service", SampleService.Name, SampleService.Name+".yaml"), SampleService.GetObjectMeta())
}
