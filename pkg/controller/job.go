package controller

import (
	"fmt"
	"github.com/sirupsen/logrus"
	apibatchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/util/workqueue"
	kubedump "kubedump/pkg"
	"os"
	"sigs.k8s.io/yaml"
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

func (handler *JobHandler) dumpJobDescription(job *apibatchv1.Job) error {
	yamlPath := resourceFilePath(kubedump.ResourceJob, handler.opts.ParentPath, job, job.Name+".yaml")

	if exists(yamlPath) {
		if err := os.Truncate(yamlPath, 0); err != nil {
			return fmt.Errorf("error truncating pod yaml file '%s' : %w", yamlPath, err)
		}
	} else {
		if err := createPathParents(yamlPath); err != nil {
			return fmt.Errorf("error creating parents for job file '%s': %s", yamlPath, err)
		}
	}

	f, err := os.OpenFile(yamlPath, os.O_WRONLY|os.O_CREATE, 0644)

	if err != nil {
		return fmt.Errorf("could not open file '%s': %w", yamlPath, err)
	}

	jobYaml, err := yaml.Marshal(job)

	if err != nil {
		return fmt.Errorf("could not marshal pod: %w", err)
	}

	_, err = f.Write(jobYaml)

	if err != nil {
		return fmt.Errorf("could not write to file '%s': %w", yamlPath, err)
	}

	return nil
}

func (handler *JobHandler) handleFunc(obj interface{}) {
	job, ok := obj.(*apibatchv1.Job)

	if !ok {
		logrus.Errorf("could not coerce object to job")
		return
	}

	if !handler.opts.Filter.Matches(job) {
		return
	}

	handler.workQueue.AddRateLimited(NewJob(func() {
		if err := handler.dumpJobDescription(job); err != nil {
			logrus.WithFields(resourceFields(job)).Errorf("could not dump job description: %s", err)
		}
	}))
}

func (handler *JobHandler) OnAdd(obj interface{}) {
	handler.handleFunc(obj)
}

func (handler *JobHandler) OnUpdate(_ interface{}, obj interface{}) {
	handler.handleFunc(obj)
}

func (handler *JobHandler) OnDelete(obj interface{}) {
	handler.handleFunc(obj)
}
