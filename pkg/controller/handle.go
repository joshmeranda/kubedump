package controller

import (
	"fmt"
	"os"
	"path"

	kubedump "github.com/joshmeranda/kubedump/pkg"
	eventsv1 "k8s.io/api/events/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type HandleKind string

const (
	HandleAdd    HandleKind = "Add"
	HandleUpdate HandleKind = "Edit"
	HandleDelete HandleKind = "Delete"
)

const (
	// [<event-time>] <type> <reason> <from> <message>
	eventFormat = "[%s] %s %s %s %s\n"
)

func (controller *Controller) handleEvent(obj any) {
	event := obj.(*eventsv1.Event)
	if event.EventTime.Time.Before(controller.startTime) {
		return
	}

	resource := kubedump.NewResourceBuilder().
		WithKind(event.Regarding.Kind).
		WithName(event.Regarding.Name).
		WithNamespace(event.Regarding.Namespace).
		Build()

	// todo: filter event by resource kind

	resourceDir := kubedump.ResourcePathBuilder{}.
		WithBase(controller.BasePath).
		WithResource(resource).
		Build()
	eventFilePath := path.Join(resourceDir, event.Regarding.Name+".events")

	if err := createPathParents(eventFilePath); err != nil {
		controller.Logger.Error(fmt.Sprintf("could not create event file '%s': %s", eventFilePath, err))
		return
	}

	eventFile, err := os.OpenFile(eventFilePath, os.O_WRONLY|os.O_CREATE, 0644)
	defer func() {
		if err := eventFile.Close(); err != nil {
			controller.Logger.Error(fmt.Sprintf("could not close event file '%s': %s", eventFilePath, err))
		}
	}()

	if err != nil {
		controller.Logger.Error(fmt.Sprintf("could not open event file '%s': %s", eventFilePath, err))
		return
	}

	s := fmt.Sprintf(eventFormat, event.EventTime, event.Type, event.Reason, event.ReportingController, event.Note)
	if _, err = eventFile.Write([]byte(s)); err != nil {
		controller.Logger.Error(fmt.Sprintf("could not write to event file '%s': %s", eventFilePath, err))
	}
}

func (controller *Controller) handlePod(handleKind HandleKind, pod kubedump.Resource, u *unstructured.Unstructured) {
	rawPod, err := controller.kubeclientset.CoreV1().Pods(pod.GetNamespace()).Get(controller.ctx, pod.GetName(), apimetav1.GetOptions{})
	if err != nil {
		controller.Logger.Error(fmt.Sprintf("could not get pod: %s", pod))
		return
	}

	switch handleKind {
	case HandleAdd:
		for _, container := range rawPod.Spec.Containers {
			controller.workQueue.AddRateLimited(NewJob(controller.ctx, JobNameAddLogStream, func() {
				stream, err := NewLogStream(LogStreamOptions{
					Pod:           rawPod,
					Container:     &container,
					Context:       controller.ctx,
					KubeClientSet: controller.kubeclientset,
					BasePath:      controller.BasePath,
					Timeout:       controller.LogSyncTimeout,
				})

				if err != nil {
					controller.Logger.Error(err.Error())
					return
				}

				logStreamId := fmt.Sprintf("%s/%s/%s", rawPod.Namespace, rawPod.Name, container.Name)

				controller.logStreamsMu.Lock()
				controller.logStreams[logStreamId] = stream
				controller.logStreamsMu.Unlock()
			}))
		}
	case HandleDelete:
		for _, container := range rawPod.Spec.Containers {
			controller.workQueue.AddRateLimited(NewJob(controller.ctx, JobNameRemoveLogStream, func() {
				logStreamId := fmt.Sprintf("%s/%s/%s", rawPod.Namespace, rawPod.Name, container.Name)

				controller.logStreamsMu.Lock()

				stream, found := controller.logStreams[logStreamId]
				if !found {
					controller.Logger.Error("bug: deleting container which isn't being streamed")
					return
				}

				if err := stream.Close(); err != nil {
					controller.Logger.Warn(err.Error())
				}

				delete(controller.logStreams, logStreamId)

				controller.logStreamsMu.Unlock()
			}))
		}
	}
}

// resourceHandlerFunc is the entrypoint for handling all resources after filtering.
func (controller *Controller) resourceHandlerFunc(handleKind HandleKind, r schema.GroupVersionResource, obj interface{}) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		controller.Logger.Error(fmt.Sprintf("received non-unstructured data: %T", obj))
		return
	}

	resource := kubedump.NewResourceBuilder().FromUnstructured(u).Build()

	if !controller.filterExpr.Matches(resource) || resource.GetKind() == "Event" {
		return
	}

	if resource.GetKind() == "Pod" {
		controller.handlePod(handleKind, resource, u)
	}

	controller.workQueue.AddRateLimited(NewJob(controller.ctx, fmt.Sprintf("%s-%s-%s-%s", JobNameDumpResourcePrefix, resource.GetKind(), resource.GetNamespace(), resource.GetName()), func() {
		dir := kubedump.ResourcePathBuilder{}.WithBase(controller.BasePath).WithResource(resource).Build()
		if err := dumpResourceDescription(path.Join(dir, resource.GetName()+".yaml"), u); err != nil {
			controller.Logger.With(
				"namespace", resource.GetNamespace(),
				"name", resource.GetName(),
			).Error(fmt.Sprintf("could not dump pod description: %s", err))
		}
	}))
}

// todo: replace interface{} with any
func (controller *Controller) onAdd(informerResource schema.GroupVersionResource, obj interface{}) {
	controller.resourceHandlerFunc(HandleAdd, informerResource, obj)
}

func (controller *Controller) onUpdate(informerResource schema.GroupVersionResource, new interface{}) {
	controller.resourceHandlerFunc(HandleUpdate, informerResource, new)
}

func (controller *Controller) onDelete(informerResource schema.GroupVersionResource, obj interface{}) {
	controller.resourceHandlerFunc(HandleDelete, informerResource, obj)
}
