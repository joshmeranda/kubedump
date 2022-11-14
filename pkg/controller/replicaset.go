package controller

import (
	"github.com/sirupsen/logrus"
	apiappsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/util/workqueue"
)

type ReplicasetHandler struct {
	// will be inherited from parent controller
	opts      Options
	workQueue workqueue.RateLimitingInterface
}

func NewReplicasetHandler(opts Options, workQueue workqueue.RateLimitingInterface) *ReplicasetHandler {
	return &ReplicasetHandler{
		opts:      opts,
		workQueue: workQueue,
	}
}

func (handler *ReplicasetHandler) handleFunc(obj interface{}, isAdd bool) {
	set, ok := obj.(*apiappsv1.ReplicaSet)

	if !ok {
		logrus.Errorf("could not coerce object to replicaset")
		return
	}

	if !handler.opts.Filter.Matches(set) {
		return
	}

	if isAdd {
		linkResourceOwners(handler.opts.ParentPath, "ReplicaSet", set)
	}

	handler.workQueue.AddRateLimited(NewJob(func() {
		if err := dumpResourceDescription(set, "ReplicaSet", handler.opts.ParentPath); err != nil {
			logrus.WithFields(resourceFields(set)).Errorf("could not dump job description: %s", err)
		}
	}))
}

func (handler *ReplicasetHandler) OnAdd(obj interface{}) {
	handler.handleFunc(obj, true)
}

func (handler *ReplicasetHandler) OnUpdate(_ interface{}, obj interface{}) {
	handler.handleFunc(obj, false)
}

func (handler *ReplicasetHandler) OnDelete(obj interface{}) {
	handler.handleFunc(obj, false)
}
