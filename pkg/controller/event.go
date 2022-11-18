package controller

import (
	"fmt"
	"github.com/sirupsen/logrus"
	eventsv1 "k8s.io/api/events/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	informersbatchv1 "k8s.io/client-go/informers/batch/v1"
	informerscorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/util/workqueue"
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
	jobInformer informersbatchv1.JobInformer
}

func NewEventHandler(opts Options, workQueue workqueue.RateLimitingInterface, podInformer informerscorev1.PodInformer, jobInformer informersbatchv1.JobInformer) *EventHandler {
	return &EventHandler{
		opts:        opts,
		workQueue:   workQueue,
		podInformer: podInformer,
		jobInformer: jobInformer,
	}
}

func (handler *EventHandler) handleResourceEvent(event *eventsv1.Event, objKind string, obj apimetav1.Object) error {
	jobDir := resourceDirPath(handler.opts.ParentPath, objKind, obj)

	eventFilePath := path.Join(jobDir, event.Regarding.Name+".events")

	if err := createPathParents(eventFilePath); err != nil {
		return fmt.Errorf("could not create job event file '%s': %w", eventFilePath, err)
	}

	eventFile, err := os.OpenFile(eventFilePath, os.O_WRONLY|os.O_CREATE, 0644)

	if err != nil {
		return fmt.Errorf("could not open job event file '%s': %w", eventFilePath, err)
	}

	s := fmt.Sprintf(eventFormat, event.EventTime, event.Type, event.Reason, event.ReportingController, event.Note)

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

	if event.EventTime.Time.Before(handler.opts.StartTime) {
		return
	}

	var err error
	switch event.Regarding.Kind {
	case "Pod":
		pod, _ := handler.podInformer.Lister().Pods(event.Regarding.Namespace).Get(event.Regarding.Name)
		if !handler.opts.Filter.Matches(pod) {
			return
		}
		err = handler.handleResourceEvent(event, event.Regarding.Kind, pod)
	case "Job":
		job, _ := handler.jobInformer.Lister().Jobs(event.Regarding.Namespace).Get(event.Regarding.Name)
		if !handler.opts.Filter.Matches(job) {
			return
		}
		err = handler.handleResourceEvent(event, event.Regarding.Kind, job)
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
