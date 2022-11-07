package tests

import (
	"context"
	"github.com/stretchr/testify/assert"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	kubedump "kubedump/pkg/cmd"
	"kubedump/tests/deployer"
	"os"
	"os/exec"
	"path"
	"testing"
	"time"
)

var kubedumpChartPath = path.Join("..", "charts", "kubedump-server")

func helmSetup(t *testing.T) (d deployer.Deployer, client kubernetes.Interface, config *rest.Config, parentPath string) {
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

func helmTeardown(t *testing.T, d deployer.Deployer, tempDir string) {
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

func TestHelm(t *testing.T) {
	d, client, config, parentPath := helmSetup(t)
	defer helmTeardown(t, d, parentPath)

	app := kubedump.NewKubedumpApp()

	err := app.Run([]string{"kubedump", "--kubeconfig", d.Kubeconfig(), "create", "--node-port", "30000", "--chart-path", kubedumpChartPath})
	assert.NoError(t, err)

	err = app.Run([]string{"kubedump", "--kubeconfig", d.Kubeconfig(), "start"})
	assert.NoError(t, err)

	deferred, err := createResources(t, client)
	assert.NoError(t, err)
	defer deferred()

	time.Sleep(5 * time.Second)

	err = app.Run([]string{"kubedump", "--kubeconfig", d.Kubeconfig(), "stop"})
	assert.NoError(t, err)

	err = app.Run([]string{"kubedump", "--kubeconfig", d.Kubeconfig(), "pull"})
	assert.NoError(t, err)

	err = app.Run([]string{"kubedump", "--kubeconfig", d.Kubeconfig(), "remove"})
	assert.NoError(t, err)

	_ = config
}
