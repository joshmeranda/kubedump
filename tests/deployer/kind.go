package deployer

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

const (
	kindExecutableName     = "kind"
	kindClusterNameBase    = "kubedump-test-kind"
	kindKubeconfigBasename = "kind-test-kubeconfig"
)

var (
	clusterAlreadyUp = fmt.Errorf("cluster already up")
	clusterDown      = fmt.Errorf("no cluster is up")
)

type KindDeployer struct {
	name       string
	kubeconfig string
	config     string
	image      string

	isUp bool
}

// NewKindDeployer create and return a Deployer along with any errors encountered.
func NewKindDeployer(config string, image string) (Deployer, error) {
	name := fmt.Sprintf("%s-%s", kindClusterNameBase, randomPostfix(10))
	kubeconfig, err := filepath.Abs("kubeconfig-" + name)

	if err != nil {
		return nil, fmt.Errorf("could not create kubeconfig path for kind deployer: %s", err)
	}

	return &KindDeployer{
		name:       name,
		kubeconfig: kubeconfig,
		config:     config,
		image:      image,
	}, nil
}

func (deployer *KindDeployer) Up() ([]byte, error) {
	if deployer.IsUp() {
		return nil, clusterAlreadyUp
	}

	args := []string{
		"create",
		"cluster",
		"--name", deployer.name,
		"--kubeconfig", deployer.kubeconfig,
		"--config", "./configs/kind.yaml",
	}

	if deployer.config != "" {
		args = append(args, "--config", deployer.config)
	}

	if deployer.image != "" {
		args = append(args, "--image", deployer.image)
	}

	cmd := exec.Command(kindExecutableName, args...)
	out, err := cmd.CombinedOutput()
	deployer.isUp = true

	return out, err
}

func (deployer *KindDeployer) IsUp() bool {
	return deployer.isUp
}

func (deployer *KindDeployer) Down() ([]byte, error) {
	if !deployer.IsUp() {
		return nil, clusterDown
	}

	cmd := exec.Command(kindExecutableName, "delete", "cluster", "--name="+deployer.Name())

	return cmd.CombinedOutput()
}

func (deployer *KindDeployer) Kubeconfig() string {
	return deployer.kubeconfig
}

func (deployer *KindDeployer) Name() string {
	return deployer.name
}

func (deployer *KindDeployer) NodeName() string {
	return fmt.Sprintf("%s-control-plane", deployer.Name())
}
