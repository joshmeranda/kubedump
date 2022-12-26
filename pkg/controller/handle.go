package controller

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	apicorev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			logrus.Errorf("could not get Pod for event: %s", err)
			return
		}
		if !controller.sieve.Matches(resource) {
			return
		}
		obj = resource.GetObjectMeta()
	case "Service":
		resource, err := controller.serviceInformer.Lister().Services(event.Regarding.Namespace).Get(event.Regarding.Name)
		if err != nil {
			logrus.Errorf("could not get Pod for event: %s", err)
			return
		}
		if !controller.sieve.Matches(resource) {
			return
		}
		obj = resource.GetObjectMeta()
	case "Job":
		resource, err := controller.jobInformer.Lister().Jobs(event.Regarding.Namespace).Get(event.Regarding.Name)
		if err != nil {
			logrus.Errorf("could not get Pod for event: %s", err)
			return
		}
		if !controller.sieve.Matches(resource) {
			return
		}
		obj = resource.GetObjectMeta()
	case "ReplicaSet":
		resource, err := controller.replicasetInformer.Lister().ReplicaSets(event.Regarding.Namespace).Get(event.Regarding.Name)
		if err != nil {
			logrus.Errorf("could not get Pod for event: %s", err)
			return
		}
		if !controller.sieve.Matches(resource) {
			return
		}
		obj = resource.GetObjectMeta()
	case "Deployment":
		resource, err := controller.deploymentInformer.Lister().Deployments(event.Regarding.Namespace).Get(event.Regarding.Name)
		if err != nil {
			logrus.Errorf("could not get Pod for event: %s", err)
			return
		}
		if !controller.sieve.Matches(resource) {
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
		logrus.Errorf("could not create job event file '%s': %s", eventFilePath, err)
	}

	eventFile, err := os.OpenFile(eventFilePath, os.O_WRONLY|os.O_CREATE, 0644)

	if err != nil {
		logrus.Errorf("could not open job event file '%s': %s", eventFilePath, err)
	}

	s := fmt.Sprintf(eventFormat, event.EventTime, event.Type, event.Reason, event.ReportingController, event.Note)

	if _, err = eventFile.Write([]byte(s)); err != nil {
		logrus.Errorf("could not write to event file '%s': %s", eventFilePath, err)
	}
}

func (controller *Controller) handlePod(kind HandleKind, pod *apicorev1.Pod) {
	switch kind {
	case HandleAdd:
		for _, container := range pod.Spec.Containers {
			controller.workQueue.AddRateLimited(NewJob(func() {
				ctx := context.TODO()

				stream, err := NewLogStream(LogStreamOptions{
					Pod:           pod,
					Container:     &container,
					Context:       ctx,
					KubeClientSet: controller.kubeclientset,
					ParentPath:    controller.ParentPath,
				})

				if err != nil {
					logrus.Errorf("%s", err)
					return
				}

				logStreamId := fmt.Sprintf("%s/%s/%s", pod.Namespace, pod.Name, container.Name)

				controller.logStreamsMu.Lock()
				controller.logStreams[logStreamId] = stream
				controller.logStreamsMu.Unlock()

				controller.workQueue.AddRateLimited(NewJob(func() {
					if err := stream.Sync(); err != nil {
						logrus.Errorf("%s", err)
					}
				}))
			}))
		}
	case HandleDelete:
		for _, container := range pod.Spec.Containers {
			controller.workQueue.AddRateLimited(NewJob(func() {
				logStreamId := fmt.Sprintf("%s/%s/%s", pod.Namespace, pod.Name, container.Name)

				controller.logStreamsMu.Lock()

				stream, found := controller.logStreams[logStreamId]

				if !found {
					logrus.Errorf("bug: deleting containr which isn't being streamed")
					return
				}

				if err := stream.Close(); err != nil {
					logrus.Warnf("%s", err)
				}

				delete(controller.logStreams, logStreamId)

				controller.logStreamsMu.Unlock()
			}))
		}
	}
}

func (controller *Controller) handleResource(kind HandleKind, handledResource HandledResource) {
	if kind == HandleAdd {
		linkResourceOwners(controller.ParentPath, handledResource.Kind, handledResource)
	}

	if matcher := selectorFromHandled(handledResource); matcher != nil {
		if err := controller.store.AddResource(handledResource, matcher); err != nil {
			logrus.Errorf("error storing '%s' label matcher: %s", handledResource.Kind, err)
		}
	}

	controller.workQueue.AddRateLimited(NewJob(func() {
		if err := dumpResourceDescription(controller.ParentPath, handledResource.Kind, handledResource); err != nil {
			logrus.WithFields(logrus.Fields{
				"namespace": handledResource.GetNamespace(),
				"name":      handledResource.GetName(),
			}).Errorf("could not dump pod description: %s", err)
		}
	}))
}

// resourceHandlerFunc is the entrypoint for handling all resources after filtering.
func (controller *Controller) resourceHandlerFunc(kind HandleKind, obj interface{}) {
	handledResource, err := NewHandledResource(kind, obj)

	if err != nil {
		logrus.Errorf("error handling %s event for type %F: %s", kind, obj, err)
		return
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

	_ = handledResource
}

func (controller *Controller) onAdd(obj interface{}) {
	controller.resourceHandlerFunc(HandleAdd, obj)
}

func (controller *Controller) onUpdate(_ interface{}, new interface{}) {
	controller.resourceHandlerFunc(HandleUpdate, new)
}

func (controller *Controller) onDelete(obj interface{}) {
	controller.resourceHandlerFunc(HandleDelete, obj)
}
