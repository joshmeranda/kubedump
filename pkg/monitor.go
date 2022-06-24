package kdump

import (
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"os"
	"reflect"
)

type EventCollector struct {
	outPath  string
	outFile  *os.File
	logger   *logrus.Logger
	watchers []watch.Interface
	stopChan chan bool
}

func NewCollector(path string, watchers []watch.Interface) (*EventCollector, error) {
	return &EventCollector{
		outPath:  path,
		outFile:  nil,
		logger:   logrus.New(),
		watchers: watchers,
		stopChan: make(chan bool),
	}, nil
}

func (collector *EventCollector) Start() error {
	//f, err := os.OpenFile(collector.outPath, os.O_WRONLY|os.O_CREATE, 0644)
	//
	//if err != nil {
	//	return fmt.Errorf("could not create collector: %w", err)
	//}
	//
	//collector.logger.SetOutput(f)

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

			obj := v.Interface().(watch.Event).Object

			switch obj.(type) {
			case *corev1.Pod:
				pod, _ := obj.(*corev1.Pod)

				logrus.WithFields(logrus.Fields{
					"resource":  "pod",
					"namespace": pod.Namespace,
					"name":      pod.Name,
					"phase":     pod.Status.Phase,
				}).Info("event received")
			case *corev1.Service:
				service, _ := obj.(*corev1.Service)

				logrus.WithFields(logrus.Fields{
					"resource":  "pod",
					"namespace": service.Namespace,
					"name":      service.Name,
				})

			case *corev1.Secret:
				secret, _ := obj.(*corev1.Secret)

				logrus.WithFields(logrus.Fields{
					"resource":  "pod",
					"namespace": secret.Namespace,
					"name":      secret.Name,
				})
			}
		}
	}()

	return nil
}

func (collector *EventCollector) Stop() error {
	//err := collector.outFile.Close()
	//
	//if err != nil {
	//	return fmt.Errorf("could not close file handle: %w", err)
	//}

	collector.outFile = nil
	collector.logger = nil

	return nil
}
