package tests

import (
	"context"
	"os"
	"os/exec"
	"path"
	"testing"
	"time"

	kubedump "github.com/joshmeranda/kubedump/pkg/cmd"
	"github.com/joshmeranda/kubedump/tests/deployer"
	"github.com/stretchr/testify/assert"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var kubedumpChartPath = path.Join("..", "charts", "kubedump-server")

func helmSetup(t *testing.T) (teardown func(), d deployer.Deployer, client kubernetes.Interface, config *rest.Config, basePath string, ctx context.Context) {
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
		basePath = path.Join(dir, "kubedump-test")
	}

	readyChan := make(chan struct{})
	wait.Until(func() {
		// todo: we should add a NodeName field to deployers, this will only work for kind as of now
		node, err := client.CoreV1().Nodes().Get(context.TODO(), d.NodeName(), apimetav1.GetOptions{})

		if err == nil {
			for _, condition := range node.Status.Conditions {
				if condition.Type == apicorev1.NodeReady && condition.Status == apicorev1.ConditionTrue {
					close(readyChan)
				}
			}
			t.Logf("node '%s' not yet ready", d.Name())
		} else {
			t.Errorf("could not get node '%s': %s", d.Name(), err)
		}
	}, 5*time.Second, readyChan)

	t.Log("node is ready")

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

func TestHelm(t *testing.T) {
	t.Skip("doesn't work consistently yet")

	teardown, d, client, _, basePath, ctx := helmSetup(t)
	defer teardown()

	app := kubedump.NewKubedumpApp()

	err := app.Run([]string{"kubedump", "--kubeconfig", d.Kubeconfig(), "create", "--node-port", "30000", "--chart-path", kubedumpChartPath})
	assert.NoError(t, err)

	err = app.Run([]string{"kubedump", "--kubeconfig", d.Kubeconfig(), "start"})
	assert.NoError(t, err)

	deferred, waiter, err := createResources(t, client, basePath, ctx)
	assert.NoError(t, err)
	defer deferred()

	waiter()

	err = app.Run([]string{"kubedump", "--kubeconfig", d.Kubeconfig(), "stop"})
	assert.NoError(t, err)

	err = app.Run([]string{"kubedump", "--kubeconfig", d.Kubeconfig(), "pull"})
	assert.NoError(t, err)

	err = app.Run([]string{"kubedump", "--kubeconfig", d.Kubeconfig(), "remove"})
	assert.NoError(t, err)
}
