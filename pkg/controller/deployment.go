package controller

import (
	"fmt"
	"github.com/sirupsen/logrus"
	apiappsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/util/workqueue"
	"os"
	"sigs.k8s.io/yaml"
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

func (handler *DeploymentHandler) dumpDeploymentDescription(deployment *apiappsv1.Deployment) error {
	yamlPath := resourceFilePath("Deployment", handler.opts.ParentPath, deployment, deployment.Name+".yaml")

	if exists(yamlPath) {
		if err := os.Truncate(yamlPath, 0); err != nil {
			return fmt.Errorf("error truncating deployment yaml file '%s' : %w", yamlPath, err)
		}
	} else {
		if err := createPathParents(yamlPath); err != nil {
			return fmt.Errorf("error creating parents for deployment file '%s': %s", yamlPath, err)
		}
	}

	f, err := os.OpenFile(yamlPath, os.O_WRONLY|os.O_CREATE, 0644)

	if err != nil {
		return fmt.Errorf("could not open file '%s': %w", yamlPath, err)
	}

	descriptionYaml, err := yaml.Marshal(deployment)

	if err != nil {
		return fmt.Errorf("could not marshal pod: %w", err)
	}

	_, err = f.Write(descriptionYaml)

	if err != nil {
		return fmt.Errorf("could not write to file '%s': %w", yamlPath, err)
	}

	return nil
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
		for _, ownerRef := range deployment.OwnerReferences {
			if err := linkToOwner(handler.opts.ParentPath, ownerRef, "Depoyment", deployment); err != nil {
				logrus.Errorf("could not link deployment to '%s' parent '%s': %s", ownerRef.Kind, ownerRef.Name, err)
			}
		}
	}

	handler.workQueue.AddRateLimited(NewJob(func() {
		if err := handler.dumpDeploymentDescription(deployment); err != nil {
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
