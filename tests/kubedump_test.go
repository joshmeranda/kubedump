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

func controllerSetup(t *testing.T) (d deployer.Deployer, client kubernetes.Interface, basePath string) {
	if found, err := exec.LookPath("kind"); err == nil {
		t.Logf("deploying cluster using 'kind' at '%s'", found)

		if d, err = deployer.NewKindDeployer("", ""); err != nil {
			t.Fatalf("could not create kind deployer: %s", err)
		}
	} else {
		t.Fatal("could not determine suitable k8s deployer")
	}

	if out, err := d.Up(); err != nil {
		t.Fatalf("could not deploy cluster: %s\nOutput:\n%s", err, out)
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
		basePath = path.Join(dir, "kubedump-test")
	}

	stopChan := make(chan struct{})
	wait.Until(func() {
		_, err := client.CoreV1().ServiceAccounts("default").Get(context.TODO(), "default", apimetav1.GetOptions{})

		if err == nil {
			close(stopChan)
			return
		}

		t.Logf("cluster '%s' not yet ready", d.Name())

	}, time.Second*5, stopChan)

	t.Logf("cluster '%s' is ready", d.Name())

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
	d, client, basePath := controllerSetup(t)
	defer controllerTeardown(t, d, basePath)

	deferred, err := createResources(t, client)
	defer deferred()
	if err != nil {
		t.Fatalf("failed to create all resources: %s", err)
		return
	}

	//block until pods are running
	stopCh := make(chan struct{})
	wait.Until(func() { checkPods(t, client, stopCh) }, time.Second*5, stopCh)
	<-stopCh

	stopChan := make(chan interface{})
	done := make(chan interface{})

	go func() {
		verbose := false
		nWorkers := fmt.Sprintf("%d", 5)
		filter := "namespace default"

		app := kubedump.NewKubedumpApp(stopChan)

		var err error
		if verbose {
			err = app.Run([]string{"kubedump", "--kubeconfig", d.Kubeconfig(), "dump", "--verbose", "--workers", nWorkers, "--destination", basePath, "--filter", filter})
		} else {
			err = app.Run([]string{"kubedump", "--kubeconfig", d.Kubeconfig(), "dump", "--workers", nWorkers, "--destination", basePath, "--filter", filter})
		}

		assert.NoError(t, err)

		close(done)

		t.Log("kubedump is finished")
	}()

	time.Sleep(30 * time.Second)

	close(stopChan)
	<-done

	assertResource(t, basePath, newHandledResourceNoErr(&SamplePod), true)

	assertResource(t, basePath, newHandledResourceNoErr(&SampleJob), true)
	assertLinkGlob(t, path.Join(basePath, SampleJob.Namespace, "job", SampleJob.Name, "pod"), glob.MustCompile(fmt.Sprintf("%s-*", SampleJob.Name)))

	assertResource(t, basePath, newHandledResourceNoErr(&SampleReplicaSet), true)

	assertResource(t, basePath, newHandledResourceNoErr(&SampleDeployment), true)
	assertLinkGlob(t, path.Join(basePath, SampleDeployment.Namespace, "deployment", SampleDeployment.Name, "replicaset"), glob.MustCompile(fmt.Sprintf("%s-*", SampleDeployment.Name)))

	assertResource(t, basePath, newHandledResourceNoErr(&SampleService), false)

	assertResource(t, basePath, newHandledResourceNoErr(&SampleConfigMap), false)

	if t.Failed() {
		copyTree(t, basePath, d.Name()+".dump")
	}
}
