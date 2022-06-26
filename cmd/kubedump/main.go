package main

import (
	"context"
	"github.com/sirupsen/logrus"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	kubedump "kubedump/pkg"
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

	podClient := client.CoreV1().Pods(kubedump.Namespace)

	podWatcher, err := podClient.Watch(context.TODO(), apismeta.ListOptions{})
	if err != nil {
		panic(err)
	}

	jobWatcher, err := client.BatchV1().Jobs(kubedump.Namespace).Watch(context.TODO(), apismeta.ListOptions{})
	if err != nil {
		panic(err)
	}

	eventWatcher, err := client.EventsV1().Events(kubedump.Namespace).Watch(context.TODO(), apismeta.ListOptions{})
	if err != nil {
		panic(err)
	}

	collector, err := kubedump.NewCollector("kubedump",
		[]watch.Interface{
			eventWatcher,
			podWatcher,
			jobWatcher,
		},
		map[string]v1.PodInterface{
			kubedump.Namespace: podClient,
		},
	)
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
