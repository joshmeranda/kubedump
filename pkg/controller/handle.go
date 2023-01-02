package controller

import (
	"fmt"
	apicorev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubedump/pkg"
	"os"
	"path"
)

const (
	// [<event-time>] <type> <reason> <from> <message>
	eventFormat = "[%s] %s %s %s %s\n"
)

func (controller *Controller) handleEvent(event *eventsv1.Event) {
	if event.EventTime.Time.Before(controller.startTime) {
		return
	}

	var obj apimetav1.Object

	switch event.Regarding.Kind {
	case "Pod":
		resource, err := controller.podInformer.Lister().Pods(event.Regarding.Namespace).Get(event.Regarding.Name)
		if err != nil {
			controller.Logger.Errorf("could not get Pod for event: %s", err)
			return
		}
		handledResource, _ := kubedump.NewHandledResource("Pod", resource)
		if !controller.sieve.Matches(handledResource) {
			return
		}
		obj = resource.GetObjectMeta()
	case "Service":
		resource, err := controller.serviceInformer.Lister().Services(event.Regarding.Namespace).Get(event.Regarding.Name)
		if err != nil {
			controller.Logger.Errorf("could not get Pod for event: %s", err)
			return
		}
		handledResource, _ := kubedump.NewHandledResource("Service", resource)
		if !controller.sieve.Matches(handledResource) {
			return
		}
		obj = resource.GetObjectMeta()
	case "Job":
		resource, err := controller.jobInformer.Lister().Jobs(event.Regarding.Namespace).Get(event.Regarding.Name)
		if err != nil {
			controller.Logger.Errorf("could not get Pod for event: %s", err)
			return
		}
		handledResource, _ := kubedump.NewHandledResource("Job", resource)
		if !controller.sieve.Matches(handledResource) {
			return
		}
		obj = resource.GetObjectMeta()
	case "ReplicaSet":
		resource, err := controller.replicasetInformer.Lister().ReplicaSets(event.Regarding.Namespace).Get(event.Regarding.Name)
		if err != nil {
			controller.Logger.Errorf("could not get Pod for event: %s", err)
			return
		}
		handledResource, _ := kubedump.NewHandledResource("ReplicaSet", resource)
		if !controller.sieve.Matches(handledResource) {
			return
		}
		obj = resource.GetObjectMeta()
	case "Deployment":
		resource, err := controller.deploymentInformer.Lister().Deployments(event.Regarding.Namespace).Get(event.Regarding.Name)
		if err != nil {
			controller.Logger.Errorf("could not get Pod for event: %s", err)
			return
		}
		handledResource, _ := kubedump.NewHandledResource("Deployment", resource)
		if !controller.sieve.Matches(handledResource) {
			return
		}
		obj = resource.GetObjectMeta()
	default:
		// unhandled event type
		return
	}

	resourceDir := resourceDirPath(controller.ParentPath, event.Regarding.Kind, obj)

	eventFilePath := path.Join(resourceDir, event.Regarding.Name+".events")

	if err := createPathParents(eventFilePath); err != nil {
		controller.Logger.Errorf("could not create job event file '%s': %s", eventFilePath, err)
	}

	eventFile, err := os.OpenFile(eventFilePath, os.O_WRONLY|os.O_CREATE, 0644)

	if err != nil {
		controller.Logger.Errorf("could not open job event file '%s': %s", eventFilePath, err)
	}

	s := fmt.Sprintf(eventFormat, event.EventTime, event.Type, event.Reason, event.ReportingController, event.Note)

	if _, err = eventFile.Write([]byte(s)); err != nil {
		controller.Logger.Errorf("could not write to event file '%s': %s", eventFilePath, err)
	}
}

func (controller *Controller) handlePod(kind kubedump.HandleKind, pod *apicorev1.Pod) {
	switch kind {
	case kubedump.HandleAdd:
		for _, container := range pod.Spec.Containers {
			controller.workQueue.AddRateLimited(NewJob(func() {
				stream, err := NewLogStream(LogStreamOptions{
					Pod:           pod,
					Container:     &container,
					Context:       controller.ctx,
					KubeClientSet: controller.kubeclientset,
					ParentPath:    controller.ParentPath,
					Timeout:       controller.LogSyncTimeout,
				})

				if err != nil {
					controller.Logger.Errorf("%s", err)
					return
				}

				logStreamId := fmt.Sprintf("%s/%s/%s", pod.Namespace, pod.Name, container.Name)

				controller.logStreamsMu.Lock()
				controller.logStreams[logStreamId] = stream
				controller.logStreamsMu.Unlock()
			}))
		}
	case kubedump.HandleDelete:
		for _, container := range pod.Spec.Containers {
			controller.workQueue.AddRateLimited(NewJob(func() {
				logStreamId := fmt.Sprintf("%s/%s/%s", pod.Namespace, pod.Name, container.Name)

				controller.logStreamsMu.Lock()

				stream, found := controller.logStreams[logStreamId]

				if !found {
					controller.Logger.Errorf("bug: deleting container which isn't being streamed")
					return
				}

				if err := stream.Close(); err != nil {
					controller.Logger.Warnf("%s", err)
				}

				delete(controller.logStreams, logStreamId)

				controller.logStreamsMu.Unlock()
			}))
		}
	}
}

func (controller *Controller) handleResource(_ kubedump.HandleKind, handledResource kubedump.HandledResource) {
	matcher, err := selectorFromHandled(handledResource)
	if err != nil {
		controller.Logger.Debugf("could not create matcher for resource '%s': %s", handledResource.String(), err)
	} else {
		controller.Logger.Debugf("adding selector for resource '%s'", handledResource.String())

		if err := controller.store.AddResource(handledResource, matcher); err != nil {
			controller.Logger.Errorf("error storing '%s' label matcher: %s", handledResource.Kind, err)
		}
	}

	controller.workQueue.AddRateLimited(NewJob(func() {
		if err := dumpResourceDescription(controller.ParentPath, handledResource); err != nil {
			controller.Logger.With(
				"namespace", handledResource.GetNamespace(),
				"name", handledResource.GetName(),
			).Errorf("could not dump pod description: %s", err)
		}
	}))
}

// resourceHandlerFunc is the entrypoint for handling all resources after filtering.
func (controller *Controller) resourceHandlerFunc(kind kubedump.HandleKind, obj interface{}) {
	handledResource, err := kubedump.NewHandledResource(kind, obj)
	if err != nil {
		controller.Logger.Errorf("error handling %s event for type %F: %s", kind, obj, err)
		return
	}

	resources, err := controller.store.GetResources(handledResource)
	if err != nil {
		controller.Logger.Errorf("error fetching resources: %s", err)
	}

	if len(resources) == 0 && !controller.sieve.Matches(handledResource) {
		return
	}

	for _, resource := range resources {
		if err := linkMatchedResource(controller.ParentPath, resource, handledResource); err != nil {
			controller.Logger.Errorf("error: %s", err)
		}
	}

	switch handledResource.Kind {
	case "Event":
		controller.handleEvent(handledResource.Resource.(*eventsv1.Event))
		return
	case "Pod":
		controller.handlePod(kind, handledResource.Resource.(*apicorev1.Pod))
		fallthrough
	case "Service", "Job", "ReplicaSet", "Deployment":
		controller.handleResource(kind, handledResource)
	default:
		panic(fmt.Sprintf("bug: unsupported resource was not caught by filter: %s (%F)", handledResource, obj))
	}
}

func (controller *Controller) onAdd(obj interface{}) {
	controller.resourceHandlerFunc(kubedump.HandleAdd, obj)
}

func (controller *Controller) onUpdate(_ interface{}, new interface{}) {
	controller.resourceHandlerFunc(kubedump.HandleUpdate, new)
}

func (controller *Controller) onDelete(obj interface{}) {
	controller.resourceHandlerFunc(kubedump.HandleDelete, obj)
}
