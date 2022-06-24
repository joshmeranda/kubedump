package kdump

import (
	"fmt"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"os"
	"path"
	"reflect"
	"sigs.k8s.io/yaml"
)

const (
	EventLogName    = "events.log"
	ResourceDirName = "resources"
)

type EventCollector struct {
	rootPath string
	outFile  *os.File
	logger   *logrus.Logger
	watchers []watch.Interface
	stopChan chan bool
}

func NewCollector(rootPath string, watchers []watch.Interface) (*EventCollector, error) {
	return &EventCollector{
		rootPath: rootPath,
		outFile:  nil,
		logger:   logrus.New(),
		watchers: watchers,
		stopChan: make(chan bool),
	}, nil
}

func (collector *EventCollector) podFields(pod *corev1.Pod) logrus.Fields {
	return logrus.Fields{
		"resource":  "pod",
		"namespace": pod.Namespace,
		"name":      pod.Name,
		"phase":     pod.Status.Phase,
	}
}

func (collector *EventCollector) jobFields(job *batchv1.Job) logrus.Fields {
	return logrus.Fields{
		"resource":  "job",
		"namespace": job.Namespace,
		"name":      job.Name,
		"":          job.Status.Conditions[0].Type,
	}
}

func (collector *EventCollector) serviceFields(service *corev1.Service) logrus.Fields {
	return logrus.Fields{
		"resource":  "pod",
		"namespace": service.Namespace,
		"name":      service.Name,
	}
}

func (collector *EventCollector) secretFields(secret *corev1.Secret) logrus.Fields {
	return logrus.Fields{
		"resource":  "pod",
		"namespace": secret.Namespace,
		"name":      secret.Name,
	}
}

func (collector *EventCollector) dumpResource(resourceType string, obj apismeta.Object) error {
	resourceFilePath := path.Join(collector.rootPath, ResourceDirName, obj.GetNamespace(), resourceType, obj.GetName())

	if err := createPathParents(resourceFilePath); err != nil {
		return fmt.Errorf("could not dump resource: %w", err)
	}

	if exists(resourceFilePath) {
		if err := os.Truncate(resourceFilePath, 0); err != nil {
			return fmt.Errorf("could not truncate existing resource file '%s': %w", resourceFilePath, err)
		}
	}

	file, err := os.OpenFile(resourceFilePath, os.O_WRONLY|os.O_CREATE, 0644)

	if err != nil {
		return fmt.Errorf("could not open resource file '%s': %w", resourceFilePath, err)
	}

	data, err := yaml.Marshal(obj)

	if err != nil {
		return fmt.Errorf("could not marshal resource to yaml: %w", err)
	}

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("could not write yaml to resource file '%s': %w", resourceFilePath, err)
	}

	return nil
}

func (collector *EventCollector) Start() error {
	eventsPath := path.Join(collector.rootPath, EventLogName)

	if err := createPathParents(eventsPath); err != nil {
		return fmt.Errorf("could not create collector: %w", err)
	}

	f, err := os.OpenFile(path.Join(collector.rootPath, EventLogName), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)

	if err != nil {
		return fmt.Errorf("could not create collector: %w", err)
	}

	collector.outFile = f
	collector.logger.SetOutput(f)

	var selectorSet []reflect.SelectCase

	for _, watcher := range collector.watchers {
		selectorSet = append(selectorSet, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(watcher.ResultChan()),
		})
	}

	go func() {
		for {
			_, v, ok := reflect.Select(selectorSet)

			if !ok {
				break
			}

			eventObj := v.Interface().(watch.Event).Object
			apiObj, ok := eventObj.(apismeta.Object)

			var fields logrus.Fields
			var resourceType string

			switch eventObj.(type) {
			case *corev1.Pod:
				pod, _ := eventObj.(*corev1.Pod)

				fields = collector.podFields(pod)
				resourceType = "pod"
			case *batchv1.Job:
				job, _ := eventObj.(*batchv1.Job)

				fields = collector.jobFields(job)
				resourceType = "job"
			case *corev1.Service:
				service, _ := eventObj.(*corev1.Service)

				fields = collector.serviceFields(service)
				resourceType = "service"
			case *corev1.Secret:
				secret, _ := eventObj.(*corev1.Secret)

				fields = collector.secretFields(secret)
				resourceType = "secret"
			}

			logrus.WithFields(fields).Info()
			if err := collector.dumpResource(resourceType, apiObj); err != nil {
				logrus.Errorf("could not dump resource: %s", err)
			}
		}
	}()

	return nil
}

func (collector *EventCollector) Stop() error {
	err := collector.outFile.Close()

	if err != nil {
		return fmt.Errorf("could not close file handle: %w", err)
	}

	collector.outFile = nil

	return nil
}
