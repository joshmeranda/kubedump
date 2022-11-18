package controller

import (
	"github.com/sirupsen/logrus"
	apiappsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/util/workqueue"
	"time"
)

// we have got to be able to generate this
func mostRecentReplicasetConditionTime(conditions []apiappsv1.ReplicaSetCondition) time.Time {
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

	if !handler.opts.Filter.Matches(set) || handler.opts.StartTime.After(mostRecentReplicasetConditionTime(set.Status.Conditions)) {
		return
	}

	if isAdd {
		linkResourceOwners(handler.opts.ParentPath, "ReplicaSet", set)
	}

	handler.workQueue.AddRateLimited(NewJob(func() {
		if err := dumpResourceDescription(handler.opts.ParentPath, "ReplicaSet", set); err != nil {
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
