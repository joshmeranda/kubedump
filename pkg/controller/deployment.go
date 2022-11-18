package controller

import (
	"github.com/sirupsen/logrus"
	apiappsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/util/workqueue"
	"time"
)

func mostRecentDeploymentConditionTime(conditions []apiappsv1.DeploymentCondition) time.Time {
	if len(conditions) == 0 {
		logrus.Warnf("encountered job with no conditions")
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
	deployment, ok := obj.(*apiappsv1.Deployment)

	if !ok {
		logrus.Errorf("could not coerse object to deployment")
		return
	}

	if !handler.opts.Filter.Matches(deployment) || handler.opts.StartTime.After(mostRecentDeploymentConditionTime(deployment.Status.Conditions)) {
		return
	}

	if isAdd {
		linkResourceOwners(handler.opts.ParentPath, "Deployment", deployment)
	}

	handler.workQueue.AddRateLimited(NewJob(func() {
		if err := dumpResourceDescription(handler.opts.ParentPath, "Deployment", deployment); err != nil {
			logrus.WithFields(resourceFields(deployment)).Errorf("could not dump deployment description: %s", err)
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
