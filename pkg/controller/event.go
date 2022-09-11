package controller

import (
	"fmt"
	"github.com/sirupsen/logrus"
	eventsv1 "k8s.io/api/events/v1"
	informerscorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/util/workqueue"
	kubedump "kubedump/pkg"
	"os"
	"path"
)

const (
	// [<event-time>] <type> <reason> <from> <message>
	eventFormat = "[%s] %s %s %s %s\n"
)

type EventHandler struct {
	// will be inherited from parent controller
	opts        Options
	workQueue   workqueue.RateLimitingInterface
	podInformer informerscorev1.PodInformer
}

func NewEventHandler(opts Options, workQueue workqueue.RateLimitingInterface, podInformer informerscorev1.PodInformer) *EventHandler {
	return &EventHandler{
		opts:        opts,
		workQueue:   workQueue,
		podInformer: podInformer,
	}
}

func (handler *EventHandler) handlePodEvent(podEvent *eventsv1.Event) error {
	// todo: filter pods
	pod, err := handler.podInformer.Lister().Pods(podEvent.Regarding.Namespace).Get(podEvent.Regarding.Name)

	if err != nil {
		return fmt.Errorf("could not get pod from event: %w", err)
	}

	podDir := resourceDirPath(kubedump.ResourcePod, handler.opts.ParentPath, pod)

	// todo: this does not account for job pods
	eventFilePath := path.Join(podDir, podEvent.Regarding.Name+".events")

	if err := createPathParents(eventFilePath); err != nil {
		return fmt.Errorf("could not create pod event file '%s': %w", eventFilePath, err)
	}

	eventFile, err := os.OpenFile(eventFilePath, os.O_WRONLY|os.O_CREATE, 0644)

	if err != nil {
		return fmt.Errorf("could not open pod event file '%s': %w", eventFilePath, err)
	}

	s := fmt.Sprintf(eventFormat, podEvent.EventTime, podEvent.Type, podEvent.Reason, podEvent.ReportingController, podEvent.Note)

	if _, err = eventFile.Write([]byte(s)); err != nil {
		return fmt.Errorf("could not write to event file '%s': %w", eventFilePath, err)
	}

	return nil
}

func (handler *EventHandler) handleFunc(obj interface{}) {
	event, ok := obj.(*eventsv1.Event)

	if !ok {
		logrus.Errorf("could not coerce object to event")
		return
	}

	var err error
	switch kubedump.ResourceKind(event.Regarding.Kind) {
	case kubedump.ResourcePod:
		err = handler.handlePodEvent(event)
	}

	if err != nil {
		logrus.Errorf("error handling event: %s", err)
	}
}

func (handler *EventHandler) OnAdd(obj interface{}) {
	handler.handleFunc(obj)
}

func (handler *EventHandler) OnUpdate(_ interface{}, obj interface{}) {
	handler.handleFunc(obj)
}

func (handler *EventHandler) OnDelete(obj interface{}) {
	handler.handleFunc(obj)
}
