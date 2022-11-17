package controller

import (
	"github.com/sirupsen/logrus"
	apibatchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/util/workqueue"
)

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
	job, ok := obj.(*apibatchv1.Job)

	if !ok {
		logrus.Errorf("could not coerce object to job")
		return
	}

	if !handler.opts.Filter.Matches(job) {
		return
	}

	if isAdd {
		linkResourceOwners(handler.opts.ParentPath, "Service", job)
	}

	handler.workQueue.AddRateLimited(NewJob(func() {
		if err := dumpResourceDescription(handler.opts.ParentPath, "Job", job); err != nil {
			logrus.WithFields(resourceFields(job)).Errorf("could not dump job description: %s", err)
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
