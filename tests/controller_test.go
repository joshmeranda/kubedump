package tests

import (
	"context"
	"fmt"
	"github.com/gobwas/glob"
	"github.com/stretchr/testify/assert"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"kubedump/pkg/controller"
	"kubedump/pkg/filter"
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

func TestController(t *testing.T) {
	d, client, parentPath := controllerSetup(t)
	defer controllerTeardown(t, d, parentPath)

	f, _ := filter.Parse("namespace default")

	opts := controller.Options{
		ParentPath: parentPath,
		Filter:     f,
	}

	time.Sleep(5 * time.Second)

	// apply objects to cluster
	deferred, err := createResources(t, client)
	assert.NoError(t, err)
	defer deferred()

	c := controller.NewController(client, opts)
	assert.NoError(t, c.Start(5))
	time.Sleep(5 * time.Second)
	assert.NoError(t, c.Stop())

	assertResourceFile(t, "Pod", path.Join(parentPath, SamplePod.Namespace, "pod", SamplePod.Name, SamplePod.Name+".yaml"), SamplePod.GetObjectMeta())

	assertResourceFile(t, "Job", path.Join(parentPath, SampleJob.Namespace, "job", SampleJob.Name, SampleJob.Name+".yaml"), SampleJob.GetObjectMeta())
	assertLinkGlob(t, path.Join(parentPath, SampleJob.Namespace, "job", SampleJob.Name, "pod"), glob.MustCompile(fmt.Sprintf("%s-*", SampleJob.Name)))

	assertResourceFile(t, "Deployment", path.Join(parentPath, SampleDeployment.Namespace, "deployment", SampleDeployment.Name, SampleDeployment.Name+".yaml"), SampleDeployment.GetObjectMeta())
	//assertLinkGlob(t, path.Join(parentPath, SampleDeployment.Namespace, "deployment", SampleDeployment.Name, "replicaset"), glob.MustCompile(fmt.Sprintf("%s-*", SampleDeployment.Name)))

	displayTree(t, parentPath)
	//copyTree(t, parentPath, d.Name()+".dump")
}
