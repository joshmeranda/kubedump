package collector

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	batchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sync"
)

type NamespaceCollector struct {
	rootPath  string
	namespace string

	pods corev1.PodInterface
	jobs batchv1.JobInterface

	podCollectors map[string]*PodCollector
	jobCollectors map[string]*JobCollector

	watchers []watch.Interface

	wg *sync.WaitGroup
}

func NewNamespaceCollector(rootPath string, namespace string, client *kubernetes.Clientset) *NamespaceCollector {
	return &NamespaceCollector{
		rootPath:      rootPath,
		namespace:     namespace,
		pods:          client.CoreV1().Pods(namespace),
		jobs:          client.BatchV1().Jobs(namespace),
		podCollectors: make(map[string]*PodCollector),
		jobCollectors: make(map[string]*JobCollector),
		wg:            &sync.WaitGroup{},
	}
}

func (collector *NamespaceCollector) collectExistingPods() {
	podList, err := collector.pods.List(context.TODO(), apismeta.ListOptions{})

	if err != nil {
		logrus.Errorf("could not fetch list of existing pods in namespace '%s': %s", collector.namespace, err)
	}

	for _, pod := range podList.Items {
		collector.podCollectors[pod.Name] = NewPodCollector(collector.rootPath, collector.pods, &pod)
	}
}

func (collector *NamespaceCollector) watchPods(watcher watch.Interface) {
	logrus.Infof("starting collectors for pods in namespace '%s'", collector.namespace)

	collector.wg.Add(1)

	c := watcher.ResultChan()

	for {
		event, ok := <-c

		if !ok {
			break
		}

		pod, ok := event.Object.(*apicorev1.Pod)

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

	collector.wg.Done()
}

func (collector *NamespaceCollector) collectExistingJobs() {
	jobList, err := collector.jobs.List(context.TODO(), apismeta.ListOptions{})

	if err != nil {
		logrus.Errorf("could not fetch list of existing jobs in namespace '%s': %s", collector.namespace, err)
	}

	for _, job := range jobList.Items {
		collector.jobCollectors[job.Name] = NewJobCollector(collector.rootPath, collector.jobs, &job)
	}
}

func (collector *NamespaceCollector) watchJobs(watcher watch.Interface) {
	logrus.Infof("starting collectors for jobs in namespace '%s'", collector.namespace)

	collector.wg.Add(1)

	c := watcher.ResultChan()

	for {
		event, ok := <-c

		if !ok {
			break
		}

		job, ok := event.Object.(*apibatchv1.Job)

		if !ok {
			logrus.Errorf("could not coerce event object to job")
		}

		switch event.Type {
		case watch.Added:
			jobCollector := NewJobCollector(collector.rootPath, collector.jobs, job)

			if err := jobCollector.Start(); err != nil {
				logrus.Errorf("could not start collector for job '%s' in namespace '%s': '%s'", job.Name, job.Namespace, err)
			} else {
				collector.jobCollectors[job.Name] = jobCollector
			}
		case watch.Deleted:
			if err := collector.podCollectors[job.Name].Stop(); err != nil {
				logrus.Errorf("error waiting for collector for job '%s' in namespace '%s' to stop: %s", job.Name, job.Namespace, err)
			}
		}
	}

	collector.wg.Done()
}

func (collector *NamespaceCollector) Start() error {
	podWatcher, err := collector.pods.Watch(context.TODO(), apismeta.ListOptions{})

	if err != nil {
		return fmt.Errorf("could not watch for pods: %w", err)
	}

	jobWatcher, err := collector.jobs.Watch(context.TODO(), apismeta.ListOptions{})

	if err != nil {
		return fmt.Errorf("could not watch for pods: %w", err)
	}

	collector.collectExistingPods()
	go collector.watchPods(podWatcher)

	collector.collectExistingJobs()
	go collector.watchJobs(jobWatcher)

	collector.watchers = []watch.Interface{
		podWatcher, jobWatcher,
	}

	return nil
}

func (collector *NamespaceCollector) Stop() error {
	logrus.Infof("stopping watchers for namespace '%s'", collector.namespace)
	for _, watcher := range collector.watchers {
		watcher.Stop()
	}

	logrus.Infof("stopping pod collectors in namespace '%s'", collector.namespace)

	for podName, podCollector := range collector.podCollectors {
		if err := podCollector.Stop(); err != nil {
			logrus.Errorf("error waiting for collector for pod '%s' in namespace '%s' to stop: %s", podName, collector.namespace, err)
		}
	}

	logrus.Infof("stopping job collectors in namespace '%s'", collector.namespace)

	for jobName, jobCollector := range collector.jobCollectors {
		if err := jobCollector.Stop(); err != nil {
			logrus.Errorf("error waiting for collector for job '%s' in '%s' to stop: %s", jobName, collector.namespace, err)
		}
	}

	collector.wg.Wait()

	return nil
}
