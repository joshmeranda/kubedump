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
	"kubedump/pkg/filter"
	"sync"
)

type NamespaceCollectorOptions struct {
	ParentPath          string
	Filter              filter.Expression
	PodCollectorOptions PodCollectorOptions
	JobCollectorOptions JobCollectorOptions
}

type NamespaceCollector struct {
	rootPath  string
	namespace *apicorev1.Namespace

	pods corev1.PodInterface
	jobs batchv1.JobInterface

	podCollectors map[string]*PodCollector
	jobCollectors map[string]*JobCollector

	watchers []watch.Interface

	wg *sync.WaitGroup

	opts NamespaceCollectorOptions
}

func NewNamespaceCollector(namespace apicorev1.Namespace, client *kubernetes.Clientset, opts NamespaceCollectorOptions) *NamespaceCollector {
	return &NamespaceCollector{
		namespace:     &namespace,
		pods:          client.CoreV1().Pods(namespace.Name),
		jobs:          client.BatchV1().Jobs(namespace.Name),
		podCollectors: make(map[string]*PodCollector),
		jobCollectors: make(map[string]*JobCollector),
		wg:            &sync.WaitGroup{},
		opts:          opts,
	}
}

func (collector *NamespaceCollector) collectExistingPods() {
	podList, err := collector.pods.List(context.TODO(), apismeta.ListOptions{})

	if err != nil {
		logrus.WithFields(resourceFields(collector.namespace)).Errorf("could not fetch list of existing pods: %s", err)
	}

	for _, pod := range podList.Items {
		if collector.opts.Filter.Evaluate(pod) {
			collector.podCollectors[pod.Name] = NewPodCollector(collector.pods, pod, collector.opts.PodCollectorOptions)
		}
	}
}

func (collector *NamespaceCollector) watchPods(watcher watch.Interface) {
	logrus.WithFields(resourceFields(collector.namespace)).Infof("starting collectors for pods in namespace")

	collector.wg.Add(1)
	defer collector.wg.Done()

	for _, podCollector := range collector.podCollectors {
		if err := podCollector.Start(); err != nil {
			logrus.WithFields(resourceFields(podCollector.pod)).Infof("could not start pod collector: %s", err)
		}
	}

	c := watcher.ResultChan()

	for {
		event, ok := <-c

		if !ok {
			break
		}

		pod, ok := event.Object.(*apicorev1.Pod)

		if !ok {
			logrus.Errorf("could not coerce event object to pod")
			continue
		} else if !collector.opts.Filter.Evaluate(*pod) {
			continue
		}

		switch event.Type {
		case watch.Added:
			if _, ok := collector.podCollectors[pod.Name]; !ok {
				podCollector := NewPodCollector(collector.pods, *pod, collector.opts.PodCollectorOptions)

				if err := podCollector.Start(); err != nil {
					logrus.WithFields(resourceFields(pod)).Errorf("could not start collector for pod: '%s'", err)
				} else {
					collector.podCollectors[pod.Name] = podCollector
				}
			}
		case watch.Deleted:
			if err := collector.podCollectors[pod.Name].Stop(); err != nil {
				logrus.WithFields(resourceFields(pod)).Errorf("error waiting for pod collector to stop: '%s'", err)
			}
		}
	}
}

func (collector *NamespaceCollector) collectExistingJobs() {
	jobList, err := collector.jobs.List(context.TODO(), apismeta.ListOptions{})

	if err != nil {
		logrus.WithFields(resourceFields(collector.namespace)).Errorf("could not fetch list of existing jobs in namespace: %s", err)
	}

	for _, job := range jobList.Items {
		if collector.opts.Filter.Evaluate(job) {
			collector.jobCollectors[job.Name] = NewJobCollector(collector.jobs, job, collector.opts.JobCollectorOptions)
		}
	}
}

func (collector *NamespaceCollector) watchJobs(watcher watch.Interface) {
	logrus.WithFields(resourceFields(collector.namespace)).Infof("starting collectors for jobs in namespace")

	collector.wg.Add(1)
	defer collector.wg.Done()

	for _, jobCollector := range collector.jobCollectors {
		if err := jobCollector.Start(); err != nil {
			logrus.WithFields(resourceFields(jobCollector.job)).Infof("could not start job collector: %s", err)
		}
	}

	c := watcher.ResultChan()

	for {
		event, ok := <-c

		if !ok {
			break
		}

		job, ok := event.Object.(*apibatchv1.Job)

		if !ok {
			logrus.Errorf("could not coerce event object to job")
		} else if !collector.opts.Filter.Evaluate(*job) {
			continue
		}

		switch event.Type {
		case watch.Added:
			if _, ok := collector.jobCollectors[job.Name]; !ok {
				jobCollector := NewJobCollector(collector.jobs, *job, collector.opts.JobCollectorOptions)

				if err := jobCollector.Start(); err != nil {
					logrus.WithFields(resourceFields(job)).Errorf("could not start collector for job: '%s'", err)
				} else {
					collector.jobCollectors[job.Name] = jobCollector
				}
			}
		case watch.Deleted:
			if err := collector.jobCollectors[job.Name].Stop(); err != nil {
				logrus.WithFields(resourceFields(job)).Errorf("error waiting for job collector to stop: '%s'", err)
			}
		}
	}
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
	logrus.WithFields(resourceFields(collector.namespace)).Infof("stopping watchers for namespace")
	for _, watcher := range collector.watchers {
		watcher.Stop()
	}

	logrus.WithFields(resourceFields(collector.namespace)).Infof("stopping pod collectors in namespace")

	for _, podCollector := range collector.podCollectors {
		if err := podCollector.Stop(); err != nil {
			logrus.WithFields(resourceFields(podCollector.pod)).Errorf("error waiting for pod collector to stop: %s", err)
		}
	}

	logrus.WithFields(resourceFields(collector.namespace)).Infof("stopping job collectors in namespace")

	for _, jobCollector := range collector.jobCollectors {
		if err := jobCollector.Stop(); err != nil {
			logrus.WithFields(resourceFields(jobCollector.job)).Errorf("error waiting for job collector to stop: %s", err)
		}
	}

	collector.wg.Wait()

	return nil
}

type MultiNamespaceCollector struct {
	collectors []*NamespaceCollector
}

func NewMultiNamespaceCollector(namespaces []*apicorev1.Namespace, client *kubernetes.Clientset, opts NamespaceCollectorOptions) *MultiNamespaceCollector {
	collectors := make([]*NamespaceCollector, len(namespaces))

	for _, ns := range namespaces {
		collectors = append(collectors, NewNamespaceCollector(*ns, client, opts))
	}

	return &MultiNamespaceCollector{
		collectors: collectors,
	}
}

func (collector *MultiNamespaceCollector) Start() error {
	for _, collector := range collector.collectors {
		if err := collector.Start(); err != nil {
			logrus.Errorf("could not collect for namespace '%s': %s", collector.namespace.Name, err)
		}
	}

	return nil
}

func (collector *MultiNamespaceCollector) Stop() error {
	for _, collector := range collector.collectors {
		if err := collector.Stop(); err != nil {
			logrus.Errorf("could not stop collecting for namespace '%s': %s", collector.namespace.Name, err)
		}
	}

	return nil
}
