package main

import (
	"context"
	"github.com/sirupsen/logrus"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
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
	jobClient := client.BatchV1().Jobs(kdump.Namespace)

	podWatcher, err := podClient.Watch(context.TODO(), apismeta.ListOptions{})
	if err != nil {
		panic(err)
	}

	jobWatcher, err := jobClient.Watch(context.TODO(), apismeta.ListOptions{})
	if err != nil {
		panic(err)
	}

	collector, err := kdump.NewCollector("kdump", []watch.Interface{podWatcher, jobWatcher})
	if err != nil {
		panic(err)
	}

	logrus.Info("starting collector...")
	err = collector.Start()
	if err != nil {
		panic(err)
	}

	logrus.Info("collecting...")

	time.Sleep(time.Second * 60)

	logrus.Info("stopping collector...")
	err = collector.Stop()
	if err != nil {
		panic(err)
	}
}
