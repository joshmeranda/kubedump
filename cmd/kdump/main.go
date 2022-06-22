package main

import (
	"context"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	kdump "kubedump/pkg"
	"os"
	"time"
)

func main() {
	kubeconfig := os.Getenv("KUBECONFIG")

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	client, err := kubernetes.NewForConfig(config)

	if err != nil {
		panic(err.Error())
	}

	podClient := client.CoreV1().Pods(kdump.Namespace)

	watcher, err := podClient.Watch(context.TODO(), apismeta.ListOptions{})
	monitor, err := kdump.MonitorPods(watcher)

	if err != nil {
		panic(err)
	}

	time.Sleep(time.Second * 10)

	monitor.Stop()
}
