package main

import (
	"context"
	"github.com/sirupsen/logrus"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	kubedump "kubedump/pkg"
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

	podClient := client.CoreV1().Pods(kubedump.Namespace)

	list, err := podClient.List(context.TODO(), apismeta.ListOptions{})

	if err != nil {
		panic(err)
	}

	var podCollectors []*kubedump.PodCollector

	for _, pod := range list.Items {
		collector, _ := kubedump.NewPodCollector("kubedump", podClient, &pod)
		podCollectors = append(podCollectors, collector)
		if err := collector.Start(); err != nil {
			logrus.Error(err)
		}
	}

	time.Sleep(time.Second * 60)

	for _, collector := range podCollectors {
		if err := collector.Stop(); err != nil {
			logrus.Error(err)
		}
	}
}
