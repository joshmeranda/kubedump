// The code in this file was generated using ./pkg/codegen, do not modify it directly

package controller

import (
	"github.com/sirupsen/logrus"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"time"
)

func mostRecentServiceConditionTime(conditions []apimetav1.Condition) time.Time {
	if len(conditions) == 0 {
		// if there are no conditions we'd rather take it than not
		return time.Now().UTC()
	}

	var t time.Time

	for _, condition := range conditions {

		if utc := condition.LastTransitionTime.UTC(); utc.After(t) {
			t = utc
		}
	}

	return t
}

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
	resource, ok := obj.(*apicorev1.Service)

	if !ok {
		logrus.Errorf("could not coerce object to Service")
		return
	}

	if !handler.opts.Filter.Matches(resource) || handler.opts.StartTime.After(mostRecentServiceConditionTime(resource.Status.Conditions)) {
		return
	}

	if isAdd {
		linkResourceOwners(handler.opts.ParentPath, "Service", resource)
	}

	handler.workQueue.AddRateLimited(NewJob(func() {
		if err := dumpResourceDescription(handler.opts.ParentPath, "Service", resource); err != nil {
			logrus.WithFields(resourceFields(resource)).Errorf("could not dump Service description: %s", err)
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
