package main

import (
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	kubedump "kubedump/pkg"
	"kubedump/pkg/collector"
	"os"
	"time"
)

func main() {
	kubeconfig := os.Getenv("KUBECONFIG")

	_ = os.Setenv(kubedump.PodRefreshIntervalEnv, "1.0")
	_ = os.Setenv(kubedump.PodLogRefreshIntervalEnv, "1.0")

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	client, err := kubernetes.NewForConfig(config)

	if err != nil {
		panic(err.Error())
	}

	namespaceCollector := collector.NewNamespaceCollector("kubedump", kubedump.Namespace, client.CoreV1().Pods(kubedump.Namespace))

	if err := namespaceCollector.Start(); err != nil {
		logrus.Errorf("could not start collector for namespace '%s' : %s", kubedump.Namespace, err)
	}

	time.Sleep(time.Second * 60)

	if err := namespaceCollector.Stop(); err != nil {
		logrus.Errorf("could stop start collector for namespace '%s' : %s", kubedump.Namespace, err)
	}
}
