package controller

import (
	"github.com/sirupsen/logrus"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
)

type ServiceHandler struct {
	// will be inherited from parent controller
	opts      Options
	workQueue workqueue.RateLimitingInterface
}

func NewServiceHandler(opts Options, workQueue workqueue.RateLimitingInterface) *ServiceHandler {
	return &ServiceHandler{
		opts:      opts,
		workQueue: workQueue,
	}
}

func (handler *ServiceHandler) handleFunc(obj interface{}, isAdd bool) {
	service, ok := obj.(*apicorev1.Service)

	if !ok {
		logrus.Errorf("could not coerce object to service")
		return
	}

	if !handler.opts.Filter.Matches(service) {
		return
	}

	if isAdd {
		for _, ownerRef := range service.OwnerReferences {
			if err := linkToOwner(handler.opts.ParentPath, ownerRef, "Service", service); err != nil {
				logrus.Errorf("could not link service to '%s' parent '%s': %s", ownerRef.Kind, ownerRef.Name, err)
			}
		}
	}

	handler.workQueue.AddRateLimited(NewJob(func() {
		if err := dumpResourceDescription(handler.opts.ParentPath, "Service", service); err != nil {
			logrus.WithFields(resourceFields(service)).Errorf("could not dump service description")
		}
	}))
}

func (handler *ServiceHandler) OnAdd(obj interface{}) {
	handler.handleFunc(obj, true)
}

func (handler *ServiceHandler) OnUpdate(_ interface{}, obj interface{}) {
	handler.handleFunc(obj, false)
}

func (handler *ServiceHandler) OnDelete(obj interface{}) {
	handler.handleFunc(obj, false)
}
