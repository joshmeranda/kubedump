package controller

import (
	"context"
	"github.com/sirupsen/logrus"
	"io"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	"os"
	"path"
	"sync"
	"time"
)

func mostRecentPodConditionTime(conditions []apicorev1.PodCondition) time.Time {
	if len(conditions) == 0 {
		logrus.Warnf("encountered job with no conditions")
	}

	var t time.Time

	for _, condition := range conditions {

		if utc := condition.LastTransitionTime.UTC(); utc.After(t) {
			t = utc
		}
	}

	return t
}

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
	handler := PodHandler{
		opts:          opts,
		workQueue:     workQueue,
		kubeclientset: kubeclientset,
		logStreams:    make(map[*os.File]io.ReadCloser),
		streamMapLock: &sync.RWMutex{},
		logBuffer:     make([]byte, 4096),
	}

	handler.syncLogStreams()

	return &handler
}

// podFileWithExt returns the file path of a pad with the given extension excluding the '.' (ex 'yaml', 'log', etc)
func (handler *PodHandler) podFileWithExt(pod *apicorev1.Pod, ext string) string {
	return path.Join(resourceDirPath(handler.opts.ParentPath, "Pod", pod), pod.Name+"."+ext)
}

func (handler *PodHandler) containerLogPath(pod *apicorev1.Pod, containerName string) string {
	return path.Join(resourceDirPath(handler.opts.ParentPath, "Pod", pod), containerName+".log")
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

// todo: handle isAdd like other handlers

func (handler *PodHandler) OnAdd(obj interface{}) {
	pod, ok := obj.(*apicorev1.Pod)

	if !ok {
		logrus.Errorf("could not coerce object to pod")
		return
	}

	if !handler.opts.Filter.Matches(*pod) || handler.opts.StartTime.After(mostRecentPodConditionTime(pod.Status.Conditions)) {
		return
	}

	linkResourceOwners(handler.opts.ParentPath, "Pod", pod)

	handler.workQueue.AddRateLimited(NewJob(func() {
		if err := dumpResourceDescription(handler.opts.ParentPath, "Pod", pod); err != nil {
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
	if handler.opts.StartTime.After(time.Now()) {
		return
	}

	pod, ok := obj.(*apicorev1.Pod)

	if !ok {
		logrus.Errorf("could not coerce object to pod")
		return
	}

	if !handler.opts.Filter.Matches(*pod) || handler.opts.StartTime.After(mostRecentPodConditionTime(pod.Status.Conditions)) {
		return
	}

	handler.workQueue.AddRateLimited(NewJob(func() {
		if err := dumpResourceDescription(handler.opts.ParentPath, "Pod", pod); err != nil {
			logrus.Errorf("could not dump pod '%s/%s': %s", pod.Namespace, pod.Name, err)
		}
	}))
}

func (handler *PodHandler) OnDelete(obj interface{}) {
	if handler.opts.StartTime.After(time.Now()) {
		return
	}

	pod, ok := obj.(*apicorev1.Pod)

	if !ok {
		logrus.Errorf("could not coerce object to pod")
		return
	}

	if !handler.opts.Filter.Matches(*pod) || handler.opts.StartTime.After(mostRecentPodConditionTime(pod.Status.Conditions)) {
		return
	}

	for _, container := range pod.Spec.Containers {
		handler.workQueue.AddRateLimited(NewJob(func() {
			handler.removeContainerStream(pod, &container)
		}))
	}
}
