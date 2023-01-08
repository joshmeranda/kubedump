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

func (controller *Controller) handleEvent(handledEvent kubedump.HandledResource) {
	event := handledEvent.Resource.(*eventsv1.Event)
	if event.EventTime.Time.Before(controller.startTime) {
		return
	}

	var handledResource kubedump.HandledResource

	switch event.Regarding.Kind {
	case "Pod":
		resource, err := controller.podInformer.Lister().Pods(event.Regarding.Namespace).Get(event.Regarding.Name)
		if err != nil {
			controller.Logger.Errorf("could not get Pod for event: %s", err)
			return
		}
		handledResource, _ = kubedump.NewHandledResource("Pod", resource)
	case "Service":
		resource, err := controller.serviceInformer.Lister().Services(event.Regarding.Namespace).Get(event.Regarding.Name)
		if err != nil {
			controller.Logger.Errorf("could not get Pod for event: %s", err)
			return
		}
		handledResource, _ = kubedump.NewHandledResource("Service", resource)
	case "Job":
		resource, err := controller.jobInformer.Lister().Jobs(event.Regarding.Namespace).Get(event.Regarding.Name)
		if err != nil {
			controller.Logger.Errorf("could not get Pod for event: %s", err)
			return
		}
		handledResource, _ = kubedump.NewHandledResource("Job", resource)
	case "ReplicaSet":
		resource, err := controller.replicasetInformer.Lister().ReplicaSets(event.Regarding.Namespace).Get(event.Regarding.Name)
		if err != nil {
			controller.Logger.Errorf("could not get Pod for event: %s", err)
			return
		}
		handledResource, _ = kubedump.NewHandledResource("ReplicaSet", resource)
	case "Deployment":
		resource, err := controller.deploymentInformer.Lister().Deployments(event.Regarding.Namespace).Get(event.Regarding.Name)
		if err != nil {
			controller.Logger.Errorf("could not get Pod for event: %s", err)
			return
		}
		handledResource, _ = kubedump.NewHandledResource("Deployment", resource)
	case "ConfigMap":
		resource, err := controller.configMapInformer.Lister().ConfigMaps(event.Regarding.Namespace).Get(event.Regarding.Name)
		if err != nil {
			controller.Logger.Errorf("could not get ConfigMap for event: %s", err)
		}
		handledResource, _ = kubedump.NewHandledResource("ConfigMap", resource)
	default:
		// unhandled event type
		return
	}

	if !controller.Filter.Matches(handledResource) {
		controller.Logger.Debugf("encountered event for unhandled kind '%s'", event.Regarding.Kind)
		return
	}

	resourceDir := resourceDirPath(controller.BasePath, event.Regarding.Kind, handledResource)
	eventFilePath := path.Join(resourceDir, event.Regarding.Name+".events")

	if err := createPathParents(eventFilePath); err != nil {
		controller.Logger.Errorf("could not create job event file '%s': %s", eventFilePath, err)
	}

	eventFile, err := os.OpenFile(eventFilePath, os.O_WRONLY|os.O_CREATE, 0644)
	defer eventFile.Close()

	if err != nil {
		controller.Logger.Errorf("could not open job event file '%s': %s", eventFilePath, err)
	}

	s := fmt.Sprintf(eventFormat, event.EventTime, event.Type, event.Reason, event.ReportingController, event.Note)

	if _, err = eventFile.Write([]byte(s)); err != nil {
		controller.Logger.Errorf("could not write to event file '%s': %s", eventFilePath, err)
	}
}

func (controller *Controller) handlePod(handledPod kubedump.HandledResource) {
	pod := handledPod.Resource.(*apicorev1.Pod)

	switch handledPod.HandleEventKind {
	case kubedump.HandleAdd:
		controller.workQueue.AddRateLimited(NewJob(func() {
			controller.Logger.Debugf("checking for config map volumes in '%s'", handledPod)

			for _, volume := range pod.Spec.Volumes {
				if volume.ConfigMap != nil {
					controller.Logger.Debugf("found config map volume in '%s'", handledPod)

					handledConfigMap, _ := kubedump.NewHandledResource(handledPod.HandleEventKind, &apicorev1.ConfigMap{
						ObjectMeta: apimetav1.ObjectMeta{
							Name:      volume.ConfigMap.Name,
							Namespace: handledPod.GetNamespace(),
						},
					})

					if err := linkResource(controller.BasePath, handledPod, handledConfigMap); err != nil {
						controller.Logger.Errorf("could not link ConfigMap to Pod: %s", err)
					}
				} else if volume.Secret != nil {
					controller.Logger.Debugf("found secret volume in '%s'", handledPod)

					handledSecret, _ := kubedump.NewHandledResource(handledPod.HandleEventKind, &apicorev1.Secret{
						ObjectMeta: apimetav1.ObjectMeta{
							Name:      volume.Secret.SecretName,
							Namespace: handledPod.GetNamespace(),
						},
					})

					if err := linkResource(controller.BasePath, handledPod, handledSecret); err != nil {
						controller.Logger.Errorf("could not link secrtr to Pod: %s", err)
					}
				}
			}
		}))

		for _, container := range pod.Spec.Containers {
			controller.workQueue.AddRateLimited(NewJob(func() {
				stream, err := NewLogStream(LogStreamOptions{
					Pod:           pod,
					Container:     &container,
					Context:       controller.ctx,
					KubeClientSet: controller.kubeclientset,
					BasePath:      controller.BasePath,
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
		if err := dumpResourceDescription(controller.BasePath, handledResource); err != nil {
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

	if len(resources) == 0 && !controller.Filter.Matches(handledResource) {
		return
	}

	for _, resource := range resources {
		if err := linkResource(controller.BasePath, resource, handledResource); err != nil {
			controller.Logger.Errorf("error: %s", err)
		}
	}

	switch handledResource.Kind {
	case "Event":
		controller.handleEvent(handledResource)
		return
	case "Pod":
		controller.handlePod(handledResource)
		fallthrough
	//case "Service", "Job", "ReplicaSet", "Deployment", "ConfigMap", "Secret":
	case "Service", "Job", "ReplicaSet", "Deployment", "ConfigMap", "Secret":
		controller.handleResource(kind, handledResource)
	default:
		controller.Logger.Errorf("bug: unsupported resource was not caught by filter: %s (%F)", handledResource, obj)
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
