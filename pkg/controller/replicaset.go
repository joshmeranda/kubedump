// The code in this file was generated using ./pkg/codegen, do not modify it directly

package controller

import (
	"github.com/sirupsen/logrus"
	apiappsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/util/workqueue"
	"time"
)

func mostRecentReplicaSetConditionTime(conditions []apiappsv1.ReplicaSetCondition) time.Time {
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

type ReplicaSetHandler struct {
	// will be inherited from parent controller
	opts      Options
	workQueue workqueue.RateLimitingInterface
}

func NewReplicaSetHandler(opts Options, workQueue workqueue.RateLimitingInterface) *ReplicaSetHandler {
	return &ReplicaSetHandler{
		opts:      opts,
		workQueue: workQueue,
	}
}

func (handler *ReplicaSetHandler) handleFunc(obj interface{}, isAdd bool) {
	resource, ok := obj.(*apiappsv1.ReplicaSet)

	if !ok {
		logrus.Errorf("could not coerce object to ReplicaSet")
		return
	}

	if !handler.opts.Filter.Matches(resource) || handler.opts.StartTime.After(mostRecentReplicaSetConditionTime(resource.Status.Conditions)) {
		return
	}

	if isAdd {
		linkResourceOwners(handler.opts.ParentPath, "ReplicaSet", resource)
	}

	handler.workQueue.AddRateLimited(NewJob(func() {
		if err := dumpResourceDescription(handler.opts.ParentPath, "ReplicaSet", resource); err != nil {
			logrus.WithFields(resourceFields(resource)).Errorf("could not dump ReplicaSet description: %s", err)
		}
	}))
}

func (handler *ReplicaSetHandler) OnAdd(obj interface{}) {
	handler.handleFunc(obj, true)
}

func (handler *ReplicaSetHandler) OnUpdate(_ interface{}, obj interface{}) {
	handler.handleFunc(obj, false)
}

func (handler *ReplicaSetHandler) OnDelete(obj interface{}) {
	handler.handleFunc(obj, false)
}
