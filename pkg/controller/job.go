// The code in this file was generated using ./pkg/codegen, do not modify it directly

package controller

import (
	"github.com/sirupsen/logrus"
	apibatchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/util/workqueue"
	"time"
)

func mostRecentJobConditionTime(conditions []apibatchv1.JobCondition) time.Time {
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

type JobHandler struct {
	// will be inherited from parent controller
	opts      Options
	workQueue workqueue.RateLimitingInterface
}

func NewJobHandler(opts Options, workQueue workqueue.RateLimitingInterface) *JobHandler {
	return &JobHandler{
		opts:      opts,
		workQueue: workQueue,
	}
}

func (handler *JobHandler) handleFunc(obj interface{}, isAdd bool) {
	resource, ok := obj.(*apibatchv1.Job)

	if !ok {
		logrus.Errorf("could not coerce object to Job")
		return
	}

	if !handler.opts.Filter.Matches(resource) || handler.opts.StartTime.After(mostRecentJobConditionTime(resource.Status.Conditions)) {
		return
	}

	if isAdd {
		linkResourceOwners(handler.opts.ParentPath, "Job", resource)
	}

	handler.workQueue.AddRateLimited(NewJob(func() {
		if err := dumpResourceDescription(handler.opts.ParentPath, "Job", resource); err != nil {
			logrus.WithFields(resourceFields(resource)).Errorf("could not dump Job description: %s", err)
		}
	}))
}

func (handler *JobHandler) OnAdd(obj interface{}) {
	handler.handleFunc(obj, true)
}

func (handler *JobHandler) OnUpdate(_ interface{}, obj interface{}) {
	handler.handleFunc(obj, false)
}

func (handler *JobHandler) OnDelete(obj interface{}) {
	handler.handleFunc(obj, false)
}
