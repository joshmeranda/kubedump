package controller

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	kubedump "kubedump/pkg"
	"os"
	"path"
	"sigs.k8s.io/yaml"
	"sync"
	"time"
)

type PodHandler struct {
	// will be inherited from parent controller
	opts          Options
	workQueue     workqueue.RateLimitingInterface
	kubeclientset kubernetes.Interface

	// logStreams is a map of a string identifier of a container (<namespace>/<pod>/<container-name>/c<container-id>) and a log stream
	logStreams    map[*os.File]io.ReadCloser
	streamMapLock *sync.RWMutex
	logBuffer     []byte
}

func NewPodHandler(opts Options, workQueue workqueue.RateLimitingInterface, kubeclientset kubernetes.Interface) *PodHandler {
	return &PodHandler{
		opts:          opts,
		workQueue:     workQueue,
		kubeclientset: kubeclientset,
		logStreams:    make(map[*os.File]io.ReadCloser),
		streamMapLock: &sync.RWMutex{},
		logBuffer:     make([]byte, 4096),
	}
}

// podFileWithExt returns the file path of a pad with the given extension excluding the '.' (ex 'yaml', 'log', etc)
func (handler *PodHandler) podFileWithExt(pod *apicorev1.Pod, ext string) string {
	return path.Join(resourceDirPath("Pod", handler.opts.ParentPath, pod), pod.Name+"."+ext)
}

func (handler *PodHandler) containerLogPath(pod *apicorev1.Pod, containerName string) string {
	return path.Join(resourceDirPath("Pod", handler.opts.ParentPath, pod), containerName+".log")
}

func (handler *PodHandler) dumpPodDescription(pod *apicorev1.Pod) error {
	yamlPath := handler.podFileWithExt(pod, "yaml")

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

func (handler *PodHandler) addContainerStream(pod *apicorev1.Pod, container *apicorev1.Container) {
	logFilePath := handler.containerLogPath(pod, container.Name)

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

	req := handler.kubeclientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &apicorev1.PodLogOptions{
		Container: container.Name,
		Follow:    true,
	})

	stream, err := req.Stream(context.TODO())

	if err != nil {
		logrus.WithFields(resourceFields(pod, container)).Errorf("could not start log stream for container: %s", err)
		return
	}

	handler.streamMapLock.Lock()
	handler.logStreams[logFile] = stream
	handler.streamMapLock.Unlock()
}

func (handler *PodHandler) removeContainerStream(pod *apicorev1.Pod, container *apicorev1.Container) {
	logFilePath := handler.containerLogPath(pod, container.Name)
	var logFile *os.File

	handler.streamMapLock.RLock()
	for file, _ := range handler.logStreams {
		if file.Name() == logFilePath {
			logFile = file
		}
	}
	handler.streamMapLock.RUnlock()

	handler.streamMapLock.Lock()
	delete(handler.logStreams, logFile)
	handler.streamMapLock.Unlock()
}

func (handler *PodHandler) syncLogStreams() {
	for file, stream := range handler.logStreams {
		readChan := make(chan int, 1)

		go func() {
			if n, err := stream.Read(handler.logBuffer); err != nil && err != io.EOF {
				logrus.Errorf("error writing logs to file '%s': %s", file.Name(), err)
			} else {
				readChan <- n
			}
		}()

		select {
		case n := <-readChan:
			if _, err := file.Write(handler.logBuffer[:n]); err != nil {
				logrus.Errorf("error writing logs to file '%s': %s", file.Name(), err)
			}
		case <-time.After(time.Millisecond):
		}
	}

	if !handler.workQueue.ShuttingDown() {
		handler.workQueue.AddRateLimited(NewJob(handler.syncLogStreams))
	}
}

func (handler *PodHandler) OnAdd(obj interface{}) {
	pod, ok := obj.(*apicorev1.Pod)

	if !ok {
		logrus.Errorf("could not coerce object to pod")
		return
	}

	if !handler.opts.Filter.Matches(*pod) {
		return
	}

	for _, ownerRef := range pod.OwnerReferences {
		if err := linkToOwner(handler.opts.ParentPath, ownerRef, kubedump.ResourcePod, pod); err != nil {
			logrus.Errorf("could not link pod to '%s' parent '%s': %s", ownerRef.Kind, ownerRef.Name, err)
		}
	}

	handler.workQueue.AddRateLimited(NewJob(func() {
		if err := handler.dumpPodDescription(pod); err != nil {
			logrus.WithFields(resourceFields(pod)).Errorf("could not dump pod description: %s", err)
		}
	}))

	for _, container := range pod.Spec.Containers {
		handler.workQueue.AddRateLimited(NewJob(func() {
			handler.addContainerStream(pod, &container)
		}))
	}
}

func (handler *PodHandler) OnUpdate(_ interface{}, obj interface{}) {
	pod, ok := obj.(*apicorev1.Pod)

	if !ok {
		logrus.Errorf("could not coerce object to pod")
		return
	}

	if !handler.opts.Filter.Matches(*pod) {
		return
	}

	handler.workQueue.AddRateLimited(NewJob(func() {
		if err := handler.dumpPodDescription(pod); err != nil {
			logrus.Errorf("could not dump pod '%s/%s': %s", pod.Namespace, pod.Name, err)
		}
	}))
}

func (handler *PodHandler) OnDelete(obj interface{}) {
	pod, ok := obj.(*apicorev1.Pod)

	if !ok {
		logrus.Errorf("could not coerce object to pod")
		return
	}

	if !handler.opts.Filter.Matches(*pod) {
		return
	}

	for _, container := range pod.Spec.Containers {
		handler.workQueue.AddRateLimited(NewJob(func() {
			handler.removeContainerStream(pod, &container)
		}))
	}
}
