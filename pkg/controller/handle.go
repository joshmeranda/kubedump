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

	resourceDir := kubedump.NewResourcePathBuilder().
		WithBase(controller.BasePath).
		WithResource(resource).
		Build()
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

func (controller *Controller) handlePod(handleKind HandleKind, pod kubedump.Resource) {
	rawPod, err := controller.kubeclientset.CoreV1().Pods(pod.GetNamespace()).Get(controller.ctx, pod.GetName(), apimetav1.GetOptions{})
	if err != nil {
		controller.Logger.Errorf("could not get pid: %s", pod)
	}

	switch handleKind {
	case HandleAdd:
		// todo: do this afterward
		controller.workQueue.AddRateLimited(NewJob(controller.ctx, func() {
			controller.Logger.Debugf("checking for config map volumes in '%s'", pod)

			for _, volume := range rawPod.Spec.Volumes {
				if volume.ConfigMap != nil {
					controller.Logger.Debugf("found config map volume in '%s'", pod)

					configMap := kubedump.NewResourceBuilder().
						WithKind("ConfigMap").
						WithName(volume.ConfigMap.Name).
						WithNamespace(pod.GetNamespace()).
						Build()

					if err != nil {
						controller.Logger.Errorf("could not create handled resource from ConfigMap: %s", err)
						continue
					}

					if err := linkResource(controller.BasePath, pod, configMap); err != nil {
						controller.Logger.Errorf("could not link ConfigMap to Pod: %s", err)
					}
				} else if volume.Secret != nil {
					controller.Logger.Debugf("found secret volume in '%s'", pod)

					secret := kubedump.NewResourceBuilder().
						WithKind("Secret").
						WithName(volume.Secret.SecretName).
						WithNamespace(pod.GetNamespace()).
						Build()
					if err != nil {
						controller.Logger.Errorf("could not create handled resource from Secret: %s", err)
						continue
					}

					if err := linkResource(controller.BasePath, pod, secret); err != nil {
						controller.Logger.Errorf("could not link secrtr to Pod: %s", err)
					}
				}
			}
		}))

		for _, container := range rawPod.Spec.Containers {
			controller.workQueue.AddRateLimited(NewJob(controller.ctx, func() {
				stream, err := NewLogStream(LogStreamOptions{
					Pod:           rawPod,
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

				logStreamId := fmt.Sprintf("%s/%s/%s", rawPod.Namespace, rawPod.Name, container.Name)

				controller.logStreamsMu.Lock()
				controller.logStreams[logStreamId] = stream
				controller.logStreamsMu.Unlock()
			}))
		}
	case HandleDelete:
		for _, container := range rawPod.Spec.Containers {
			controller.workQueue.AddRateLimited(NewJob(controller.ctx, func() {
				logStreamId := fmt.Sprintf("%s/%s/%s", rawPod.Namespace, rawPod.Name, container.Name)

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

// resourceHandlerFunc is the entrypoint for handling all resources after filtering.
func (controller *Controller) resourceHandlerFunc(handleKind HandleKind, r schema.GroupVersionResource, obj interface{}) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		panic("bug: received non-unstructured data")
	}
	resource := kubedump.NewResourceBuilder().FromUnstructured(u).Build()

	resources, err := controller.store.GetResources(resource)
	if err != nil {
		controller.Logger.Errorf("error fetching resources: %s", err)
	}

	if len(resources) == 0 && !controller.filterExpr.Matches(resource) || resource.GetKind() == "Event" {
		return
	}

	for _, r := range resources {
		if err := linkResource(controller.BasePath, r, resource); err != nil {
			controller.Logger.Errorf("error: %s", err)
		}
	}

	if resource.GetKind() == "Pod" {
		controller.handlePod(handleKind, resource)
	}

	matcher, err := selectorFromUnstructured(u)
	if err != nil {
		controller.Logger.Debugf("could not create matcher for resource '%s': %s", resource.String(), err)
	} else {
		controller.Logger.Debugf("adding selector for resource '%s'", resource.String())

		if err := controller.store.AddResource(resource, matcher); err != nil {
			controller.Logger.Errorf("error storing '%s' label matcher: %s", resource.GetKind(), err)
		}
	}

	controller.workQueue.AddRateLimited(NewJob(controller.ctx, func() {
		dir := kubedump.NewResourcePathBuilder().WithBase(controller.BasePath).WithResource(resource).Build()
		if err := dumpResourceDescription(path.Join(dir, resource.GetName()+".yaml"), u); err != nil {
			controller.Logger.With(
				"namespace", resource.GetNamespace(),
				"name", resource.GetName(),
			).Errorf("could not dump pod description: %s", err)
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
