package collector

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	apicorev1 "k8s.io/api/core/v1"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	kubedump "kubedump/pkg"
	"os"
	"sigs.k8s.io/yaml"
	"strconv"
	"sync"
	"time"
)

type PodCollector struct {
	rootPath                 string
	pod                      *apicorev1.Pod
	podClient                corev1.PodInterface
	lastSyncedTransitionTime time.Time

	collecting bool
	wg         *sync.WaitGroup
}

func NewPodCollector(rootPath string, podClient corev1.PodInterface, pod *apicorev1.Pod) *PodCollector {
	return &PodCollector{
		rootPath:   rootPath,
		pod:        pod,
		podClient:  podClient,
		collecting: false,
		wg:         &sync.WaitGroup{},
	}
}

func (collector *PodCollector) dumpCurrentPod() error {
	yamlPath := podYamlPath(collector.rootPath, collector.pod)

	if exists(yamlPath) {
		if err := os.Truncate(yamlPath, 0); err != nil {
			return fmt.Errorf("error truncating pod yaml file '%s' : %w", yamlPath, err)
		}
	} else {
		if err := createPathParents(yamlPath); err != nil {
			return fmt.Errorf("error creating parents for job file '%s': %s", yamlPath, err)
		}
	}

	f, err := os.OpenFile(yamlPath, os.O_WRONLY|os.O_CREATE, 0644)

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

	// todo: all similar logs should have descriptive fields (namespace, name, etc)
	logrus.WithFields(resourceFields(collector.pod)).Infof("collecting description for pod")

	for collector.collecting {
		pod, err := collector.podClient.Get(context.TODO(), collector.pod.Name, apismeta.GetOptions{})

		if err != nil {
			logrus.WithFields(resourceFields(collector.pod)).Errorf("could not get pod: %s", err)
			continue
		}

		newestTransition := mostRecentPodTransitionTime(pod.Status.Conditions)

		if newestTransition.After(collector.lastSyncedTransitionTime) {
			collector.pod = pod
			collector.lastSyncedTransitionTime = newestTransition

			if err := collector.dumpCurrentPod(); err != nil {
				logrus.Errorf("%s", err)
			}
		}

		time.Sleep(podRefreshDuration)
	}

	logrus.WithFields(resourceFields(collector.pod)).Infof("stopping description for pod")

	collector.wg.Done()
}

func (collector *PodCollector) collectLogs(logRefreshDuration time.Duration, container apicorev1.Container) {
	logFilePath := podLogsPath(collector.rootPath, collector.pod, container.Name)

	if err := createPathParents(logFilePath); err != nil {
		logrus.WithFields(resourceFields(collector.pod)).WithFields(resourceFields(container)).Errorf("could not create log file '%s': %s", logFilePath, err)
		return
	}

	logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE, 0644)

	if err != nil {
		logrus.WithFields(resourceFields(collector.pod)).WithFields(resourceFields(container)).Errorf("could not create log file '%s': %s", logFilePath, err)
		return
	}

	req := collector.podClient.GetLogs(collector.pod.Name, &apicorev1.PodLogOptions{
		Container: container.Name,
		Follow:    true,
	})

	stream, err := req.Stream(context.TODO())

	if err != nil {
		// todo: fails when container is still in "ContainerCreating"
		logrus.WithFields(resourceFields(collector.pod)).WithFields(resourceFields(container)).Errorf("could not start log stream for container: %s", err)
		return
	}

	buffer := make([]byte, 4098)

	collector.wg.Add(1)

	logrus.WithFields(resourceFields(collector.pod)).WithFields(resourceFields(container)).Infof("collecting logs for container")

	for collector.collecting {
		n, err := stream.Read(buffer)

		if err == io.EOF {
			logrus.WithFields(resourceFields(collector.pod)).WithFields(resourceFields(container)).Infof("encountered EOF on log stream for container")
			break
		} else if err != nil {
			logrus.WithFields(resourceFields(collector.pod)).WithFields(resourceFields(container)).Errorf("could not read from log stream for container: %s", err)
			break
		}

		if _, err := logFile.Write(buffer[:n]); err != nil {
			logrus.WithFields(resourceFields(collector.pod)).WithFields(resourceFields(container)).Errorf("could not write to container log file '%s': %s", logFilePath, err)
			break
		}

		time.Sleep(logRefreshDuration)
	}

	logrus.WithFields(resourceFields(collector.pod)).WithFields(resourceFields(container)).Infof("stopping logs for container")

	collector.wg.Done()
}

func (collector *PodCollector) Start() error {
	podDirPath := podDirPath(collector.rootPath, collector.pod)

	if err := createPathParents(podDirPath); err != nil {
		return fmt.Errorf("could not create collector: %w", err)
	}

	podRefreshInterval, err := strconv.ParseFloat(os.Getenv(kubedump.PodRefreshIntervalEnv), 64)

	if err != nil {
		return fmt.Errorf("could not parse env '%s' to float64: %w", kubedump.PodRefreshIntervalEnv, err)
	}

	podRefreshDuration := time.Duration(float64(time.Second) * podRefreshInterval)

	podLogRefreshInterval, err := strconv.ParseFloat(os.Getenv(kubedump.PodLogRefreshIntervalEnv), 64)

	if err != nil {
		return fmt.Errorf("could not parse env '%s' to float64: %w", kubedump.PodRefreshIntervalEnv, err)
	}

	podLogRefreshDuration := time.Duration(float64(time.Second) * podLogRefreshInterval)

	collector.collecting = true

	go collector.collectDescription(podRefreshDuration)

	for _, cnt := range collector.pod.Spec.Containers {
		go collector.collectLogs(podLogRefreshDuration, cnt)
	}

	return nil
}

func (collector *PodCollector) Stop() error {
	collector.collecting = false

	collector.wg.Wait()

	return nil
}
