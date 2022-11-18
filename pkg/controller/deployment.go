// The code in this file was generated using ./pkg/codegen, do not modify it directly

package controller

import (
	"github.com/sirupsen/logrus"
	apiappsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/util/workqueue"
	"time"
)

func mostRecentDeploymentConditionTime(conditions []apiappsv1.DeploymentCondition) time.Time {
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

type DeploymentHandler struct {
	// will be inherited from parent controller
	opts      Options
	workQueue workqueue.RateLimitingInterface
}

func NewDeploymentHandler(opts Options, workQueue workqueue.RateLimitingInterface) *DeploymentHandler {
	return &DeploymentHandler{
		opts:      opts,
		workQueue: workQueue,
	}
}

func (handler *DeploymentHandler) handleFunc(obj interface{}, isAdd bool) {
	resource, ok := obj.(*apiappsv1.Deployment)

	if !ok {
		logrus.Errorf("could not coerce object to Deployment")
		return
	}

	if !handler.opts.Filter.Matches(resource) || handler.opts.StartTime.After(mostRecentDeploymentConditionTime(resource.Status.Conditions)) {
		return
	}

	if isAdd {
		linkResourceOwners(handler.opts.ParentPath, "Deployment", resource)
	}

	handler.workQueue.AddRateLimited(NewJob(func() {
		if err := dumpResourceDescription(handler.opts.ParentPath, "Deployment", resource); err != nil {
			logrus.WithFields(resourceFields(resource)).Errorf("could not dump Deployment description: %s", err)
		}
	}))
}

func (handler *DeploymentHandler) OnAdd(obj interface{}) {
	handler.handleFunc(obj, true)
}

func (handler *DeploymentHandler) OnUpdate(_ interface{}, obj interface{}) {
	handler.handleFunc(obj, false)
}

func (handler *DeploymentHandler) OnDelete(obj interface{}) {
	handler.handleFunc(obj, false)
}
