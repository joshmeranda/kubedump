package controller

import (
	"fmt"
	"github.com/sirupsen/logrus"
	eventsv1 "k8s.io/api/events/v1"
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

func (handler *EventHandler) handlePodEvent(podEvent *eventsv1.Event) error {
	pod, err := handler.podInformer.Lister().Pods(podEvent.Regarding.Namespace).Get(podEvent.Regarding.Name)

	if err != nil {
		return fmt.Errorf("could not get pod from event: %w", err)
	}

	if !handler.opts.Filter.Matches(pod) {
		return nil
	}

	podDir := resourceDirPath(handler.opts.ParentPath, handler.opts.ParentPath, pod)

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

func (handler *EventHandler) handleJobEvent(jobEvent *eventsv1.Event) error {
	job, err := handler.jobInformer.Lister().Jobs(jobEvent.Regarding.Namespace).Get(jobEvent.Regarding.Name)

	if err != nil {
		return fmt.Errorf("could not get job from event: %s", err)
	}

	if !handler.opts.Filter.Matches(job) {
		return nil
	}

	jobDir := resourceDirPath(handler.opts.ParentPath, "Job", job)

	eventFilePath := path.Join(jobDir, jobEvent.Regarding.Name+".events")

	if err := createPathParents(eventFilePath); err != nil {
		return fmt.Errorf("could not create job event file '%s': %w", eventFilePath, err)
	}

	eventFile, err := os.OpenFile(eventFilePath, os.O_WRONLY|os.O_CREATE, 0644)

	if err != nil {
		return fmt.Errorf("could not open job event file '%s': %w", eventFilePath, err)
	}

	s := fmt.Sprintf(eventFormat, jobEvent.EventTime, jobEvent.Type, jobEvent.Reason, jobEvent.ReportingController, jobEvent.Note)

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
	switch event.Regarding.Kind {
	case "Pod":
		err = handler.handlePodEvent(event)
	case "Job":
		err = handler.handleJobEvent(event)
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
