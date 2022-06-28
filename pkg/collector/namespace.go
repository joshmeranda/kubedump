package collector

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type NamespaceCollector struct {
	rootPath      string
	namespace     string
	pods          corev1.PodInterface
	podCollectors map[string]*PodCollector
	running       bool
}

func NewNamespaceCollector(rootPath string, namespace string, client corev1.PodInterface) *NamespaceCollector {
	return &NamespaceCollector{
		rootPath:      rootPath,
		namespace:     namespace,
		pods:          client,
		podCollectors: make(map[string]*PodCollector),
		running:       false,
	}
}

func (collector *NamespaceCollector) collectExistingPods() {
	podList, err := collector.pods.List(context.TODO(), apismeta.ListOptions{})

	if err != nil {
		logrus.Errorf("could not fetch lis of existing pods in namespace '%s': %s", collector.namespace, err)
	}

	for _, pod := range podList.Items {
		collector.podCollectors[pod.Name] = NewPodCollector(collector.rootPath, collector.pods, &pod)
	}
}

func (collector *NamespaceCollector) watchPods(watcher watch.Interface) {
	logrus.Infof("starting collectors for pods in namespace '%s'", collector.namespace)

	c := watcher.ResultChan()

	for collector.running {
		event, ok := <-c

		if !ok {
			break
		}

		pod, ok := event.Object.(*v1.Pod)

		if !ok {
			logrus.Errorf("could not coerce event object to pod")
		}

		switch event.Type {
		case watch.Added:
			podCollector := NewPodCollector(collector.rootPath, collector.pods, pod)

			if err := podCollector.Start(); err != nil {
				logrus.Errorf("could not start collector for pod '%s' in namespace '%s': '%s'", pod.Name, pod.Namespace, err)
			} else {
				collector.podCollectors[pod.Name] = podCollector
			}
		case watch.Deleted:
			if err := collector.podCollectors[pod.Name].Stop(); err != nil {
				logrus.Errorf("error waiting for collector for pod '%s' in namespace '%s' to stop: %s", pod.Name, pod.Namespace, err)
			}
		}
	}
}

func (collector *NamespaceCollector) Start() error {
	podWatcher, err := collector.pods.Watch(context.TODO(), apismeta.ListOptions{})

	if err != nil {
		return fmt.Errorf("could not watch for pods: %w", err)
	}

	collector.running = true

	collector.collectExistingPods()
	go collector.watchPods(podWatcher)

	return nil
}

func (collector *NamespaceCollector) Stop() error {
	collector.running = false

	logrus.Infof("stopping pod collectors in namespace '%s'", collector.namespace)

	for podName, podCollector := range collector.podCollectors {
		if err := podCollector.Stop(); err != nil {
			logrus.Errorf("error waiting for collector for pod '%s' in namespace '%s' to stop: %s", podName, collector.namespace, err)
		}
	}

	return nil
}
