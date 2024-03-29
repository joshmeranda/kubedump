package tests

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"testing"
	"time"

	"github.com/gobwas/glob"
	kubedump "github.com/joshmeranda/kubedump/pkg"
	kubedumpcmd "github.com/joshmeranda/kubedump/pkg/cmd"
	"github.com/joshmeranda/kubedump/tests/deployer"
	"github.com/stretchr/testify/assert"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func controllerSetup(t *testing.T) (teardown func(), d deployer.Deployer, client kubernetes.Interface, ctx context.Context, basePath string) {
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

	basePath = path.Join(t.TempDir(), "kubedump-test")

	stopChan := make(chan struct{})
	wait.Until(func() {
		_, err := client.CoreV1().ServiceAccounts("default").Get(context.Background(), "default", apimetav1.GetOptions{})

		if err == nil {
			close(stopChan)
			return
		}

		t.Logf("cluster '%s' not yet ready", d.Name())

	}, time.Second*5, stopChan)

	t.Logf("cluster '%s' is ready", d.Name())

	ctx, cancel := context.WithCancel(context.Background())
	teardown = func() {
		cancel()

		if t.Failed() {
			dumpDir := t.Name() + ".dump"
			t.Logf("copying dump directory int '%s' for failed test", dumpDir)

			if err := os.RemoveAll(dumpDir); err != nil && !os.IsNotExist(err) {
				t.Errorf("error removing existing test dump: %s", err)
			}

			if err := CopyTree(basePath, dumpDir); err != nil {
				t.Errorf("%s", err)
			}
		}

		if err := os.Remove(d.Kubeconfig()); err != nil {
			t.Logf("failed to delete temporary test kubeconfig '%s': %s", d.Kubeconfig(), err)
		}

		if out, err := d.Down(); err != nil {
			t.Logf("failed to delete cluster: %s\nOutput\n%s", err, out)
		}
	}

	return
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

// todo: sometimes fails with no event for sample-pod
func TestDumpWithCluster(t *testing.T) {
	teardown, d, client, ctx, basePath := controllerSetup(t)
	defer teardown()

	deferred, waiter, err := createResources(t, client, basePath, ctx)
	defer deferred()
	if err != nil {
		t.Fatalf("failed to create all resources: %s", err)
		return
	}

	//block until pods are running
	stopCh := make(chan struct{})
	wait.Until(func() { checkPods(t, client, stopCh) }, time.Second*5, stopCh)
	<-stopCh

	done := make(chan interface{})

	go func() {
		verbose := false
		nWorkers := fmt.Sprintf("%d", 5)
		filter := "namespace default"

		app := kubedumpcmd.NewKubedumpApp()

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

	waiter()

	<-done

	AssertResource(t, basePath, kubedump.NewResourceBuilder().FromObject(SamplePod.ObjectMeta).FromType(SamplePod.TypeMeta).Build(), true)

	AssertResource(t, basePath, kubedump.NewResourceBuilder().FromObject(SampleJob.ObjectMeta).FromType(SampleJob.TypeMeta).Build(), true)
	AssertLinkGlob(t, path.Join(basePath, SampleJob.Namespace, "Job", SampleJob.Name, "Pod"), glob.MustCompile(fmt.Sprintf("%s-*", SampleJob.Name)))

	AssertResource(t, basePath, kubedump.NewResourceBuilder().FromObject(SampleReplicaSet.ObjectMeta).FromType(SampleReplicaSet.TypeMeta).Build(), true)

	AssertResource(t, basePath, kubedump.NewResourceBuilder().FromObject(SampleDeployment.ObjectMeta).FromType(SampleDeployment.TypeMeta).Build(), true)
	AssertLinkGlob(t, path.Join(basePath, SampleDeployment.Namespace, "Deployment", SampleDeployment.Name, "ReplicaSet"), glob.MustCompile(fmt.Sprintf("%s-*", SampleDeployment.Name)))

	AssertResource(t, basePath, kubedump.NewResourceBuilder().FromObject(SampleService.ObjectMeta).FromType(SampleService.TypeMeta).Build(), false)

	AssertResource(t, basePath, kubedump.NewResourceBuilder().FromObject(SampleConfigMap.ObjectMeta).FromType(SampleConfigMap.TypeMeta).Build(), false)

	AssertResource(t, basePath, kubedump.NewResourceBuilder().FromObject(SamplePodWithConfigMapVolume.ObjectMeta).FromType(SamplePodWithConfigMapVolume.TypeMeta).Build(), false)
	AssertResourceIsLinked(t, basePath,
		kubedump.NewResourceBuilder().FromObject(SamplePodWithConfigMapVolume.ObjectMeta).FromType(SamplePodWithConfigMapVolume.TypeMeta).Build(),
		kubedump.NewResourceBuilder().FromObject(SampleConfigMap.ObjectMeta).FromType(SampleConfigMap.TypeMeta).Build())

	AssertResource(t, basePath, kubedump.NewResourceBuilder().FromObject(SampleSecret.ObjectMeta).FromType(SampleSecret.TypeMeta).Build(), false)

	AssertResource(t, basePath, kubedump.NewResourceBuilder().FromObject(SamplePodWithSecretVolume.ObjectMeta).FromType(SamplePodWithSecretVolume.TypeMeta).Build(), false)
	AssertResourceIsLinked(t, basePath,
		kubedump.NewResourceBuilder().FromObject(SamplePodWithSecretVolume.ObjectMeta).FromType(SamplePodWithSecretVolume.TypeMeta).Build(),
		kubedump.NewResourceBuilder().FromObject(SampleSecret.ObjectMeta).FromType(SampleSecret.TypeMeta).Build())

	t.Fail()
}
