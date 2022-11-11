package controller

import (
	"fmt"
	"github.com/sirupsen/logrus"
	apiappsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/util/workqueue"
	"os"
	"sigs.k8s.io/yaml"
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

func (handler *ReplicasetHandler) dumpDescription(replicaset *apiappsv1.ReplicaSet) error {
	yamlPath := resourceFilePath("ReplicaSet", handler.opts.ParentPath, replicaset, replicaset.Name+".yaml")

	if exists(yamlPath) {
		if err := os.Truncate(yamlPath, 0); err != nil {
			return fmt.Errorf("error truncating replicaset yaml file '%s' : %w", yamlPath, err)
		}
	} else {
		if err := createPathParents(yamlPath); err != nil {
			return fmt.Errorf("error creating parents for replicaset file '%s': %s", yamlPath, err)
		}
	}

	f, err := os.OpenFile(yamlPath, os.O_WRONLY|os.O_CREATE, 0644)

	if err != nil {
		return fmt.Errorf("could not open file '%s': %w", yamlPath, err)
	}

	jobYaml, err := yaml.Marshal(replicaset)

	if err != nil {
		return fmt.Errorf("could not marshal pod: %w", err)
	}

	_, err = f.Write(jobYaml)

	if err != nil {
		return fmt.Errorf("could not write to file '%s': %w", yamlPath, err)
	}

	return nil
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
		for _, ownerRef := range set.OwnerReferences {
			if err := linkToOwner(handler.opts.ParentPath, ownerRef, "ReplicaSet", set); err != nil {
				logrus.Errorf("could not link replicaset '%s' parent '%s': %s", ownerRef.Kind, ownerRef.Name, err)
			}
		}
	}

	handler.workQueue.AddRateLimited(NewJob(func() {
		if err := handler.dumpDescription(set); err != nil {
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
