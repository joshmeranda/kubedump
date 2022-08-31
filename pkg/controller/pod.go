package controller

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	apicorev1 "k8s.io/api/core/v1"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubedump "kubedump/pkg"
	"os"
	"path"
	"sigs.k8s.io/yaml"
)

func (controller *Controller) podDir(pod *apicorev1.Pod) string {
	jobName, ok := pod.Labels["job-name"]

	if !ok {
		return resourceDirPath(kubedump.ResourcePod, controller.opts.ParentPath, pod)
	}

	return path.Join(resourceDirPath(kubedump.ResourceJob, controller.opts.ParentPath, &apismeta.ObjectMeta{
		Name:      jobName,
		Namespace: pod.Namespace,
	}), "pod", pod.Name)
}

// podFileWithExt returns the file path of a pad with the given extension excluding the '.' (ex 'yaml', 'log', etc)
func (controller *Controller) podFileWithExt(pod *apicorev1.Pod, ext string) string {
	return path.Join(controller.podDir(pod), pod.Name+"."+ext)
}

func (controller *Controller) containerLogPath(pod *apicorev1.Pod, containerName string) string {
	return path.Join(controller.podDir(pod), containerName+".log")
}

func (controller *Controller) dumpPodDescription(pod *apicorev1.Pod) error {
	yamlPath := controller.podFileWithExt(pod, "yaml")

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

	podYaml, err := yaml.Marshal(pod)

	if err != nil {
		return fmt.Errorf("could not marshal pod: %w", err)
	}

	_, err = f.Write(podYaml)

	if err != nil {
		return fmt.Errorf("could not write to file '%s': %w", yamlPath, err)
	}

	return nil
}

func (controller *Controller) addContainerStream(pod *apicorev1.Pod, container *apicorev1.Container) {
	logFilePath := controller.containerLogPath(pod, container.Name)

	if err := createPathParents(logFilePath); err != nil {
		logrus.WithFields(resourceFields(pod)).Errorf("could not create log file '%s': %s", logFilePath, err)
		return
	}

	logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE, 0644)

	if err != nil {
		logrus.WithFields(resourceFields(pod)).Errorf("could open create log file '%s': %s", logFilePath, err)
		return
	}

	//defer logFile.Close()

	req := controller.kubeclientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &apicorev1.PodLogOptions{
		Container: container.Name,
		Follow:    true,
	})

	stream, err := req.Stream(context.TODO())

	if err != nil {
		logrus.WithFields(resourceFields(pod, container)).Errorf("could not start log stream for container: %s", err)
		return
	}

	controller.streamMapLock.Lock()
	controller.logStreams[logFile] = stream
	controller.streamMapLock.Unlock()
}

func (controller *Controller) removeContainerStream(pod *apicorev1.Pod, container *apicorev1.Container) {
	logFilePath := controller.containerLogPath(pod, container.Name)
	var logFile *os.File

	controller.streamMapLock.RLock()
	for file, _ := range controller.logStreams {
		if file.Name() == logFilePath {
			logFile = file
		}
	}
	controller.streamMapLock.RUnlock()

	controller.streamMapLock.Lock()
	delete(controller.logStreams, logFile)
	controller.streamMapLock.Unlock()
}

func (controller *Controller) podAddHandler(obj interface{}) {
	pod, ok := obj.(*apicorev1.Pod)

	if !ok {
		logrus.Errorf("could not coerce object to pod")
		return
	}

	if !controller.opts.Filter.Matches(*pod) {
		return
	}

	controller.workQueue.AddRateLimited(NewJob(func() {
		if err := controller.dumpPodDescription(pod); err != nil {
			logrus.Errorf("could not dump pod '%s/%s': %s", pod.Namespace, pod.Name, err)
		}
	}))

	for _, container := range pod.Spec.Containers {
		controller.workQueue.AddRateLimited(NewJob(func() {
			controller.addContainerStream(pod, &container)
		}))
	}
}

func (controller *Controller) podUpdateHandler(_ interface{}, obj interface{}) {
	pod, ok := obj.(*apicorev1.Pod)

	if !ok {
		logrus.Errorf("could not coerce object to pod")
		return
	}

	if !controller.opts.Filter.Matches(*pod) {
		return
	}

	controller.workQueue.AddRateLimited(NewJob(func() {
		if err := controller.dumpPodDescription(pod); err != nil {
			logrus.Errorf("could not dump pod '%s/%s': %s", pod.Namespace, pod.Name, err)
		}
	}))
}

func (controller *Controller) podDeletedHandler(obj interface{}) {
	pod, ok := obj.(*apicorev1.Pod)

	if !ok {
		logrus.Errorf("could not coerce object to pod")
		return
	}

	if !controller.opts.Filter.Matches(*pod) {
		return
	}

	for _, container := range pod.Spec.Containers {
		controller.workQueue.AddRateLimited(NewJob(func() {
			controller.removeContainerStream(pod, &container)
		}))
	}
}
