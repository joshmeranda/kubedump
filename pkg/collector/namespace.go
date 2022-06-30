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

type NamespaceCollectorOptions struct {
	ParentPath          string
	PodCollectorOptions PodCollectorOptions
	JobCollectorOptions JobCollectorOptions
}

type NamespaceCollector struct {
	rootPath  string
	namespace string

	pods corev1.PodInterface
	jobs batchv1.JobInterface

	podCollectors map[string]*PodCollector
	jobCollectors map[string]*JobCollector

	watchers []watch.Interface

	wg *sync.WaitGroup

	opts NamespaceCollectorOptions
}

func NewNamespaceCollector(namespace string, client *kubernetes.Clientset, opts NamespaceCollectorOptions) *NamespaceCollector {
	return &NamespaceCollector{
		namespace:     namespace,
		pods:          client.CoreV1().Pods(namespace),
		jobs:          client.BatchV1().Jobs(namespace),
		podCollectors: make(map[string]*PodCollector),
		jobCollectors: make(map[string]*JobCollector),
		wg:            &sync.WaitGroup{},
		opts:          opts,
	}
}

func (collector *NamespaceCollector) collectExistingPods() {
	podList, err := collector.pods.List(context.TODO(), apismeta.ListOptions{})

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"namespace": collector.namespace,
		}).Errorf("could not fetch list of existing pods: %s", err)
	}

	for _, pod := range podList.Items {
		collector.podCollectors[pod.Name] = NewPodCollector(collector.pods, &pod, collector.opts.PodCollectorOptions)
	}
}

func (collector *NamespaceCollector) watchPods(watcher watch.Interface) {
	logrus.WithFields(logrus.Fields{
		"namespace": collector.namespace,
	}).Infof("starting collectors for pods in namespace")

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
			podCollector := NewPodCollector(collector.pods, pod, collector.opts.PodCollectorOptions)

			if err := podCollector.Start(); err != nil {
				logrus.WithFields(resourceFields(pod)).Errorf("could not start collector for pod: '%s'", err)
			} else {
				collector.podCollectors[pod.Name] = podCollector
			}
		case watch.Deleted:
			if err := collector.podCollectors[pod.Name].Stop(); err != nil {
				logrus.WithFields(resourceFields(pod)).Errorf("error waiting for pod collector to stop: '%s'", err)
			}
		}
	}

	collector.wg.Done()
}

func (collector *NamespaceCollector) collectExistingJobs() {
	jobList, err := collector.jobs.List(context.TODO(), apismeta.ListOptions{})

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"namespace": collector.namespace,
		}).Errorf("could not fetch list of existing jobs in namespace: %s", err)
	}

	for _, job := range jobList.Items {
		collector.jobCollectors[job.Name] = NewJobCollector(collector.jobs, &job, collector.opts.JobCollectorOptions)
	}
}

func (collector *NamespaceCollector) watchJobs(watcher watch.Interface) {
	logrus.WithFields(logrus.Fields{
		"namespace": collector.namespace,
	}).Infof("starting collectors for jobs in namespace")

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
			jobCollector := NewJobCollector(collector.jobs, job, collector.opts.JobCollectorOptions)

			if err := jobCollector.Start(); err != nil {
				logrus.WithFields(resourceFields(job)).Errorf("could not start collector for job: '%s'", err)
			} else {
				collector.jobCollectors[job.Name] = jobCollector
			}
		case watch.Deleted:
			if err := collector.jobCollectors[job.Name].Stop(); err != nil {
				logrus.WithFields(resourceFields(job)).Errorf("error waiting for job collector to stop: '%s'", err)
			}
		}
	}

	collector.wg.Done()
}

func (collector *NamespaceCollector) Start() error {
	podWatcher, err := collector.pods.Watch(context.TODO(), apismeta.ListOptions{})

	if err != nil {
		return fmt.Errorf("could not watch for pods '%s': %w", collector.namespace, err)
	}

	jobWatcher, err := collector.jobs.Watch(context.TODO(), apismeta.ListOptions{})

	if err != nil {
		return fmt.Errorf("could not watch for pods '%s': %w", collector.namespace, err)
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
	logrus.WithFields(logrus.Fields{
		"namespace": collector.namespace,
	}).Infof("stopping watchers for namespace")
	for _, watcher := range collector.watchers {
		watcher.Stop()
	}

	logrus.WithFields(logrus.Fields{
		"namespace": collector.namespace,
	}).Infof("stopping pod collectors in namespace")

	for _, podCollector := range collector.podCollectors {
		if err := podCollector.Stop(); err != nil {
			logrus.WithFields(resourceFields(podCollector.pod)).Errorf("error waiting for pod collector to stop: %s", err)
		}
	}

	logrus.WithFields(logrus.Fields{
		"namespace": collector.namespace,
	}).Infof("stopping job collectors in namespace")

	for _, jobCollector := range collector.jobCollectors {
		if err := jobCollector.Stop(); err != nil {
			logrus.WithFields(resourceFields(jobCollector.job)).Errorf("error waiting for job collector to stop: %s", err)
		}
	}

	collector.wg.Wait()

	return nil
}
