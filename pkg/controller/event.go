package controller

import (
	"fmt"
	"github.com/sirupsen/logrus"
	eventsv1 "k8s.io/api/events/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	opts      Options
	workQueue workqueue.RateLimitingInterface
}

func NewEventHandler(opts Options, workQueue workqueue.RateLimitingInterface) *EventHandler {
	return &EventHandler{
		opts:      opts,
		workQueue: workQueue,
	}
}

func (handler *EventHandler) handlePodEvent(podEvent *eventsv1.Event) error {
	podDir := resourceDirPath(kubedump.ResourcePod, handler.opts.ParentPath, &v1.ObjectMeta{
		Name:      podEvent.Regarding.Name,
		Namespace: podEvent.Regarding.Namespace,
	})

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

	logrus.Debugf("Received event: %s", s)

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
		logrus.Errorf("error handlin pod event: %s", err)
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
