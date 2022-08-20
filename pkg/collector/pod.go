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
	"path"
	"sigs.k8s.io/yaml"
	"sync"
	"time"
)

type PodCollectorOptions struct {
	ParentPath          string
	LogInterval         time.Duration
	DescriptionInterval time.Duration
}

type PodCollector struct {
	pod                      *apicorev1.Pod
	podClient                corev1.PodInterface
	lastSyncedTransitionTime time.Time

	collecting bool
	wg         *sync.WaitGroup

	opts PodCollectorOptions

	files map[string]*os.File
}

func NewPodCollector(podClient corev1.PodInterface, pod apicorev1.Pod, opts PodCollectorOptions) *PodCollector {
	return &PodCollector{
		pod:        &pod,
		podClient:  podClient,
		collecting: false,
		wg:         &sync.WaitGroup{},

		opts:  opts,
		files: make(map[string]*os.File),
	}
}

func (collector *PodCollector) podDir() string {
	jobName, ok := collector.pod.Labels["job-name"]

	if !ok {
		return resourceDirPath(kubedump.ResourcePod, collector.opts.ParentPath, collector.pod)
	}

	return path.Join(resourceDirPath(kubedump.ResourceJob, collector.opts.ParentPath, &apismeta.ObjectMeta{
		Name:      jobName,
		Namespace: collector.pod.Namespace,
	}), "pod", collector.pod.Name)
}

func (collector *PodCollector) podYaml() string {
	return path.Join(collector.podDir(), collector.pod.Name+".yaml")
}

func (collector *PodCollector) podLog() string {
	return path.Join(collector.podDir(), collector.pod.Name+".log")
}

func (collector *PodCollector) dumpCurrentPod() error {
	yamlPath := collector.podYaml()

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

func (collector *PodCollector) collectDescription() {
	collector.wg.Add(1)
	defer collector.wg.Done()

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

		time.Sleep(collector.opts.DescriptionInterval)
	}

	logrus.WithFields(resourceFields(collector.pod)).Infof("stopping description for pod")
}

// waitContainer waits for the given container to be ready in the pod or until collecting is stop, each check is delayed
// by the log collection interval.
func (collector *PodCollector) waitContainer(container apicorev1.Container) {
	for collector.collecting {
		for _, status := range collector.pod.Status.ContainerStatuses {
			if status.Name == container.Name && status.Ready {
				return
			}
		}

		time.Sleep(collector.opts.LogInterval)
	}
}

func (collector *PodCollector) collectLogs(container apicorev1.Container) {
	collector.waitContainer(container)

	logFilePath := collector.podLog()

	if err := createPathParents(logFilePath); err != nil {
		logrus.WithFields(resourceFields(collector.pod, container)).Errorf("could not create log file '%s': %s", logFilePath, err)
		return
	}

	logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE, 0644)

	if err != nil {
		logrus.WithFields(resourceFields(collector.pod, container)).Errorf("could not create log file '%s': %s", logFilePath, err)
		return
	}

	collector.files[logFilePath] = logFile
	defer logFile.Close()

	req := collector.podClient.GetLogs(collector.pod.Name, &apicorev1.PodLogOptions{
		Container: container.Name,
		Follow:    true,
	})

	stream, err := req.Stream(context.TODO())

	if err != nil {
		logrus.WithFields(resourceFields(collector.pod, container)).Errorf("could not start log stream for container: %s", err)
		return
	}

	defer stream.Close()

	buffer := make([]byte, 4098)

	collector.wg.Add(1)
	defer collector.wg.Done()

	logrus.WithFields(resourceFields(collector.pod, container)).Infof("collecting logs for container")

	time.Sleep(collector.opts.LogInterval)

	for collector.collecting {
		n, err := stream.Read(buffer)

		if err == io.EOF {
			logrus.WithFields(resourceFields(collector.pod, container)).Debugf("encountered EOF on log stream for container")
		} else if err != nil {
			logrus.WithFields(resourceFields(collector.pod, container)).Errorf("could not read from log stream for container: %s", err)
			break
		} else if _, err := logFile.Write(buffer[:n]); err != nil {
			logrus.WithFields(resourceFields(collector.pod, container)).Errorf("could not write to container log file '%s': %s", logFilePath, err)
			break
		}

		// EOF encountered or log written to file
		time.Sleep(collector.opts.LogInterval)
	}

	delete(collector.files, logFilePath)

	logrus.WithFields(resourceFields(collector.pod, container)).Infof("stopping logs for container")
}

func (collector *PodCollector) Start() error {
	podDirPath := resourceDirPath(kubedump.ResourcePod, collector.opts.ParentPath, collector.pod)

	if err := createPathParents(podDirPath); err != nil {
		return fmt.Errorf("could not create collector: %w", err)
	}

	collector.collecting = true

	go collector.collectDescription()

	for _, cnt := range collector.pod.Spec.Containers {
		go collector.collectLogs(cnt)
	}

	return nil
}

func (collector *PodCollector) Sync() error {
	syncFailed := false

	for filePath, file := range collector.files {
		if err := file.Sync(); err != nil {
			logrus.Errorf("error syncing logs to '%s'", filePath)
			syncFailed = true
		} else {
			logrus.Debugf("synced pod '%s' logs to '%s'", collector.pod.Name, filePath)
		}
	}

	if syncFailed {
		return fmt.Errorf("could not sync pod '%s', see logs for details", collector.pod.Name)
	}

	return nil
}

func (collector *PodCollector) Stop() error {
	collector.collecting = false

	collector.wg.Wait()

	return nil
}
