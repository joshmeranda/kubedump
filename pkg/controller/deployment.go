package controller

import (
	"github.com/sirupsen/logrus"
	apiappsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/util/workqueue"
)

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

	if !handler.opts.Filter.Matches(deployment) {
		return
	}

	if isAdd {
		linkResourceOwners(handler.opts.ParentPath, "Deployment", deployment)
	}

	handler.workQueue.AddRateLimited(NewJob(func() {
		if err := dumpResourceDescription(deployment, "Deployment", handler.opts.ParentPath); err != nil {
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
