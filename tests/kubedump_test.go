package tests

import (
	"context"
	"fmt"
	"github.com/gobwas/glob"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
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

func TestDump(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)

	d, client, parentPath := controllerSetup(t)
	defer controllerTeardown(t, d, parentPath)

	stopChan := make(chan interface{})

	go func() {
		app := kubedump.NewKubedumpApp(stopChan)

		err := app.Run([]string{"kubedump", "--kubeconfig", d.Kubeconfig(), "dump", "--destination", parentPath, "--filter", "namespace default"})
		assert.NoError(t, err)
	}()

	deferred, err := createResources(t, client)
	assert.NoError(t, err)
	defer deferred()

	time.Sleep(5 * time.Second)
	close(stopChan)

	assertResourceFile(t, "Pod", path.Join(parentPath, SamplePod.Namespace, "pod", SamplePod.Name, SamplePod.Name+".yaml"), SamplePod.GetObjectMeta())

	assertResourceFile(t, "Job", path.Join(parentPath, SampleJob.Namespace, "job", SampleJob.Name, SampleJob.Name+".yaml"), SampleJob.GetObjectMeta())
	assertLinkGlob(t, path.Join(parentPath, SampleJob.Namespace, "job", SampleJob.Name, "pod"), glob.MustCompile(fmt.Sprintf("%s-*", SampleJob.Name)))

	assertResourceFile(t, "ReplicaSet", path.Join(parentPath, SampleReplicaSet.Namespace, "replicaset", SampleReplicaSet.Name, SampleReplicaSet.Name+".yaml"), SampleReplicaSet.GetObjectMeta())

	assertResourceFile(t, "Deployment", path.Join(parentPath, SampleDeployment.Namespace, "deployment", SampleDeployment.Name, SampleDeployment.Name+".yaml"), SampleDeployment.GetObjectMeta())
	assertLinkGlob(t, path.Join(parentPath, SampleDeployment.Namespace, "deployment", SampleDeployment.Name, "replicaset"), glob.MustCompile(fmt.Sprintf("%s-*", SampleDeployment.Name)))

	assertResourceFile(t, "Service", path.Join(parentPath, SampleService.Namespace, "service", SampleService.Name, SampleService.Name+".yaml"), SampleService.GetObjectMeta())

	//displayTree(t, parentPath)
	//copyTree(t, parentPath, d.Name()+".dump")
}
