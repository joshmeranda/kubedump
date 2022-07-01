package collector

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	apicorev1 "k8s.io/api/core/v1"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

type ClusterCollectorOptions struct {
	ParentPath                string
	NamespaceCollectorOptions NamespaceCollectorOptions
}

type ClusterCollector struct {
	client *kubernetes.Clientset

	namespaceCollectors map[string]*NamespaceCollector

	opts ClusterCollectorOptions
}

func NewClusterCollector(client *kubernetes.Clientset, opts ClusterCollectorOptions) *ClusterCollector {
	return &ClusterCollector{
		client:              client,
		opts:                opts,
		namespaceCollectors: map[string]*NamespaceCollector{},
	}
}

func (collector *ClusterCollector) collectExistingNamespaces() {
	namespaces, err := collector.client.CoreV1().Namespaces().List(context.TODO(), apismeta.ListOptions{})

	if err != nil {
		logrus.Errorf("could not fetch list of namespaces: %s", err)
	}

	for _, namespace := range namespaces.Items {
		collector.namespaceCollectors[namespace.Name] = NewNamespaceCollector(namespace, collector.client, collector.opts.NamespaceCollectorOptions)
	}
}

func (collector *ClusterCollector) watchNamespaces(watcher watch.Interface) {
	logrus.Infof("starting collectors for namespaces in cluster")

	for _, namespaceCollector := range collector.namespaceCollectors {
		if err := namespaceCollector.Start(); err != nil {
			logrus.WithFields(logrus.Fields{
				"namespace": namespaceCollector.namespace,
			}).Infof("could not start collector for namespace: %s", err)
		}
	}

	c := watcher.ResultChan()

	for {
		event, ok := <-c

		if !ok {
			break
		}

		namespace, ok := event.Object.(*apicorev1.Namespace)

		if !ok {
			logrus.Errorf("could not coerce event object to namespace")
		}

		switch event.Type {
		case watch.Added:
			if _, ok := collector.namespaceCollectors[namespace.Name]; !ok {
				namespaceCollector := NewNamespaceCollector(*namespace, collector.client, collector.opts.NamespaceCollectorOptions)

				if err := namespaceCollector.Start(); err != nil {
					logrus.WithFields(resourceFields(namespace)).Errorf("could not start collector for namespace: '%s'", err)
				} else {
					collector.namespaceCollectors[namespace.Name] = namespaceCollector
				}
			}
		case watch.Deleted:
			if err := collector.namespaceCollectors[namespace.Name].Stop(); err != nil {
				logrus.WithFields(resourceFields(namespace)).Errorf("error waiting for namespace collector to stop: '%s'", err)
			}
		}
	}
}

func (collector *ClusterCollector) Start() error {
	watcher, err := collector.client.CoreV1().Namespaces().Watch(context.TODO(), apismeta.ListOptions{})

	if err != nil {
		return fmt.Errorf("could not watch namespaces on cluster: %s", err)
	}

	collector.collectExistingNamespaces()
	go collector.watchNamespaces(watcher)

	return nil
}

func (collector *ClusterCollector) Stop() error {
	logrus.Infof("stopping collecting for cluster")

	for _, namespaceCollector := range collector.namespaceCollectors {
		if err := namespaceCollector.Stop(); err != nil {
			logrus.WithFields(logrus.Fields{
				"namespace": namespaceCollector.namespace,
			}).Error("could not stop collector for namespace")
		}
	}

	return nil
}
