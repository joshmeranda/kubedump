package kubedump

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	corev1 "k8s.io/api/core/v1"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"os"
	"sigs.k8s.io/yaml"
	"strconv"
	"sync"
	"time"
)

const (
	EventLogName    = "events.log"
	ResourceDirName = "resources"
)

type Collector interface {
	// Start will start collecting operations.
	Start() error

	// Stop stops and waits for all collecting operations are finish.
	Stop() error
}

type PodCollector struct {
	rootPath                 string
	pod                      *corev1.Pod
	podName                  string
	podClient                v1.PodInterface
	lastSyncedTransitionTime time.Time

	collecting bool
	wg         *sync.WaitGroup
}

func NewPodCollector(rootPath string, podClient v1.PodInterface, pod *corev1.Pod) (*PodCollector, error) {
	return &PodCollector{
		rootPath:   rootPath,
		pod:        pod,
		podClient:  podClient,
		collecting: false,
		wg:         &sync.WaitGroup{},
	}, nil
}

func (collector *PodCollector) dumpCurrentPod() error {
	yamlPath := PodYamlPath(collector.rootPath, collector.pod)

	if exists(yamlPath) {
		if err := os.Truncate(yamlPath, 0); err != nil {
			return fmt.Errorf("error truncating pod ymal file '%s' : %w", yamlPath, err)
		}
	}

	f, err := os.OpenFile(PodYamlPath(collector.rootPath, collector.pod), os.O_WRONLY|os.O_CREATE, 0644)

	if err != nil {
		return fmt.Errorf("could not open file '%s': %w", yamlPath, err)
	}

	podYaml, err := yaml.Marshal(collector.pod)

	if err != nil {
		return fmt.Errorf("could not marshal pod: %w", err)
	}

	_, err = f.Write(podYaml)

	if err != nil {
		return fmt.Errorf("could not write to file '%s': %w", yamlPath, err)
	}

	return nil
}

func (collector *PodCollector) collectDescription(podRefreshDuration time.Duration) {
	collector.wg.Add(1)

	logrus.Infof("collecting description for pod '%s'", collector.pod.Name)

	for collector.collecting {
		pod, err := collector.podClient.Get(context.TODO(), collector.pod.Name, apismeta.GetOptions{})

		if err != nil {
			logrus.Errorf("could not get pod '%s' in '%s': %s", collector.pod.Name, collector.pod.Namespace, err)
		}

		newestTransition := MostRecentTransitionTime(pod.Status.Conditions)

		if newestTransition.After(collector.lastSyncedTransitionTime) {
			collector.pod = pod
			collector.lastSyncedTransitionTime = newestTransition

			if err := collector.dumpCurrentPod(); err != nil {
				logrus.Errorf("%s", err)
			}
		}

		time.Sleep(podRefreshDuration)
	}

	logrus.Infof("stopping description for pod '%s'", collector.pod.Name)

	collector.wg.Done()
}

func (collector *PodCollector) collectLogs(logRefreshDuration time.Duration, containerName string) {
	logFilePath := PodLogsPath(collector.rootPath, collector.pod, containerName)

	if err := createPathParents(logFilePath); err != nil {
		logrus.Errorf("could not create log file '%s' for container '%s' on pod '%s': %s", logFilePath, containerName, collector.pod.Name, err)
		return
	}

	logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE, 0644)

	if err != nil {
		logrus.Errorf("could not create log file '%s' for container '%s' on pod '%s': %s", logFilePath, containerName, collector.pod.Name, err)
		return
	}

	req := collector.podClient.GetLogs(collector.pod.Name, &corev1.PodLogOptions{
		Container: containerName,
		Follow:    true,
	})

	stream, err := req.Stream(context.TODO())

	if err != nil {
		logrus.Errorf("could not start log stream for container '%s' on pod '%s': %s", containerName, collector.pod.Name, err)
		return
	}

	buffer := make([]byte, 4098)

	collector.wg.Add(1)

	logrus.Infof("collecting logs for container '%s' on pod '%s'", containerName, collector.pod.Name)

	for collector.collecting {
		n, err := stream.Read(buffer)

		if err == io.EOF {
			logrus.Infof("encountered EOF on log stream for container '%s' on pod '%s'", containerName, collector.pod.Name)
			break
		} else if err != nil {
			logrus.Errorf("could not read from log stream for container '%s' on pod '%s': %s", containerName, collector.pod.Name, err)
			break
		}

		if _, err := logFile.Write(buffer[:n]); err != nil {
			logrus.Errorf("could not write to log file '%s' for container '%s' on pod '%s': %s", logFilePath, containerName, collector.pod.Name, err)
			break
		}

		time.Sleep(logRefreshDuration)
	}

	logrus.Infof("stopping logs for container '%s' on pod '%s'", containerName, collector.pod.Name)

	collector.wg.Done()
}

func (collector *PodCollector) Start() error {
	podDirPath := PodDirPath(collector.rootPath, collector.pod)

	if err := createPathParents(podDirPath); err != nil {
		return fmt.Errorf("could not create collector: %w", err)
	}

	podRefreshInterval, err := strconv.ParseFloat(os.Getenv(PodRefreshIntervalEnv), 64)

	if err != nil {
		return fmt.Errorf("could not parse env '%s' to float64: %w", PodRefreshIntervalEnv, err)
	}

	podRefreshDuration := time.Duration(float64(time.Second) * podRefreshInterval)

	podLogRefreshInterval, err := strconv.ParseFloat(os.Getenv(PodLogRefreshIntervalEnv), 64)

	if err != nil {
		return fmt.Errorf("could not parse env '%s' to float64: %w", PodRefreshIntervalEnv, err)
	}

	podLogRefreshDuration := time.Duration(float64(time.Second) * podLogRefreshInterval)

	collector.collecting = true

	go collector.collectDescription(podRefreshDuration)

	for _, cnt := range collector.pod.Status.ContainerStatuses {
		go collector.collectLogs(podLogRefreshDuration, cnt.Name)
	}

	return nil
}

func (collector *PodCollector) Stop() error {
	collector.collecting = false

	collector.wg.Wait()

	return nil
}

//type WatcherCollector struct {
//	rootPath   string
//	outFile    *os.File
//	logger     *logrus.Logger
//	watchers   []watch.Interface
//	stopChan   chan bool
//	podClients map[string]v1.PodInterface
//}
//
//func NewCollector(rootPath string, watchers []watch.Interface, podClients map[string]v1.PodInterface) (*WatcherCollector, error) {
//	return &WatcherCollector{
//		rootPath:   rootPath,
//		outFile:    nil,
//		logger:     logrus.New(),
//		watchers:   watchers,
//		stopChan:   make(chan bool),
//		podClients: podClients,
//	}, nil
//}
//
//func (collector *WatcherCollector) podFields(pod *corev1.Pod) logrus.Fields {
//	return logrus.Fields{
//		"resource":  "pod",
//		"namespace": pod.Namespace,
//		"name":      pod.Name,
//		"phase":     pod.Status.Phase,
//	}
//}
//
//func (collector *WatcherCollector) jobFields(job *batchv1.Job) logrus.Fields {
//	return logrus.Fields{
//		"resource":  "job",
//		"namespace": job.Namespace,
//		"name":      job.Name,
//	}
//}
//
//func (collector *WatcherCollector) serviceFields(service *corev1.Service) logrus.Fields {
//	return logrus.Fields{
//		"resource":  "pod",
//		"namespace": service.Namespace,
//		"name":      service.Name,
//	}
//}
//
//func (collector *WatcherCollector) secretFields(secret *corev1.Secret) logrus.Fields {
//	return logrus.Fields{
//		"resource":  "pod",
//		"namespace": secret.Namespace,
//		"name":      secret.Name,
//	}
//}
//
//func (collector *WatcherCollector) eventFields(event *eventv1.Event) logrus.Fields {
//	return logrus.Fields{
//		"type":   event.Type,
//		"reason": event.Reason,
//		"target": event.Regarding.Name,
//		"note":   event.Note,
//	}
//}
//
//func (collector *WatcherCollector) dumpResource(resourceType string, obj apismeta.Object) error {
//	resourceFilePath := path.Join(collector.rootPath, ResourceDirName, obj.GetNamespace(), resourceType, obj.GetName()) + ".yaml"
//
//	if err := createPathParents(resourceFilePath); err != nil {
//		return fmt.Errorf("could not dump resource: %w", err)
//	}
//
//	if exists(resourceFilePath) {
//		if err := os.Truncate(resourceFilePath, 0); err != nil {
//			return fmt.Errorf("could not truncate existing resource file '%s': %w", resourceFilePath, err)
//		}
//	}
//
//	file, err := os.OpenFile(resourceFilePath, os.O_WRONLY|os.O_CREATE, 0644)
//
//	if err != nil {
//		return fmt.Errorf("could not open resource file '%s': %w", resourceFilePath, err)
//	}
//
//	data, err := yaml.Marshal(obj)
//
//	if err != nil {
//		return fmt.Errorf("could not marshal resource to yaml: %w", err)
//	}
//
//	if _, err := file.Write(data); err != nil {
//		return fmt.Errorf("could not write yaml to resource file '%s': %w", resourceFilePath, err)
//	}
//
//	return nil
//}
//
//func (collector *WatcherCollector) Start() error {
//	eventsPath := path.Join(collector.rootPath, EventLogName)
//
//	if err := createPathParents(eventsPath); err != nil {
//		return fmt.Errorf("could not create collector: %w", err)
//	}
//
//	f, err := os.OpenFile(path.Join(collector.rootPath, EventLogName), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
//
//	if err != nil {
//		return fmt.Errorf("could not create collector: %w", err)
//	}
//
//	collector.outFile = f
//	collector.logger.SetOutput(f)
//
//	selectorSet := []reflect.SelectCase{
//		{
//			Dir:  reflect.SelectRecv,
//			Chan: reflect.ValueOf(collector.stopChan),
//		},
//	}
//
//	for _, watcher := range collector.watchers {
//		selectorSet = append(selectorSet, reflect.SelectCase{
//			Dir:  reflect.SelectRecv,
//			Chan: reflect.ValueOf(watcher.ResultChan()),
//		})
//	}
//
//	go func() {
//		for {
//			i, v, ok := reflect.Select(selectorSet)
//			_ = ok
//
//			if i == 0 {
//				break
//			}
//
//			event := v.Interface().(watch.Event)
//			eventObj := event.Object
//
//			var fields logrus.Fields
//
//			if clusterEvent, ok := eventObj.(*eventv1.Event); ok {
//				fields = collector.eventFields(clusterEvent)
//				collector.logger.WithFields(fields).Info()
//			} else {
//				var resourceType string
//
//				switch eventObj.(type) {
//				case *corev1.Pod:
//					pod, _ := eventObj.(*corev1.Pod)
//
//					fields = collector.podFields(pod)
//					resourceType = "pod"
//				case *batchv1.Job:
//					job, _ := eventObj.(*batchv1.Job)
//
//					fields = collector.jobFields(job)
//					resourceType = "job"
//				case *corev1.Service:
//					service, _ := eventObj.(*corev1.Service)
//
//					fields = collector.serviceFields(service)
//					resourceType = "service"
//				case *corev1.Secret:
//					secret, _ := eventObj.(*corev1.Secret)
//
//					fields = collector.secretFields(secret)
//					resourceType = "secret"
//				case *eventv1.Event:
//				default:
//					logrus.Infof("unhandled type: %s", reflect.TypeOf(eventObj))
//				}
//
//				if apiObj, ok := eventObj.(apismeta.Object); !ok {
//					logrus.Errorf("could not coerce value to api object: %s", reflect.TypeOf(apiObj))
//				} else if err := collector.dumpResource(resourceType, apiObj); err != nil {
//					logrus.Errorf("could not dump resource: %s", err)
//				}
//			}
//		}
//	}()
//
//	return nil
//}
//
//func (collector *WatcherCollector) Stop() error {
//	err := collector.outFile.Close()
//
//	if err != nil {
//		return fmt.Errorf("could not close file handle: %w", err)
//	}
//
//	collector.stopChan <- true
//	for _, watcher := range collector.watchers {
//		watcher.Stop()
//	}
//
//	collector.outFile = nil
//
//	return nil
//}
