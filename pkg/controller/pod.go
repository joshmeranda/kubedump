package controller

import (
	"fmt"
	"github.com/sirupsen/logrus"
	apicorev1 "k8s.io/api/core/v1"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubedump "kubedump/pkg"
	"os"
	"path"
	"sigs.k8s.io/yaml"
)

func (controller *Controller) podDir(pod *apicorev1.Pod) string {
	jobName, ok := pod.Labels["job-name"]

	if !ok {
		return resourceDirPath(kubedump.ResourcePod, controller.opts.ParentPath, pod)
	}

	return path.Join(resourceDirPath(kubedump.ResourceJob, controller.opts.ParentPath, &apismeta.ObjectMeta{
		Name:      jobName,
		Namespace: pod.Namespace,
	}), "pod", pod.Name)
}

// podFileWithExt returns the file path of a pad with the given extension excluding the '.' (ex 'yaml', 'log', etc)
func (controller *Controller) podFileWithExt(pod *apicorev1.Pod, ext string) string {
	return path.Join(controller.podDir(pod), pod.Name+"."+ext)
}

func (controller *Controller) dumpPodDescription(pod *apicorev1.Pod) error {
	logrus.Debugf("dumping description for pod '%s/%s'", pod.Namespace, pod.Name)

	yamlPath := controller.podFileWithExt(pod, "yaml")

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

	podYaml, err := yaml.Marshal(pod)

	if err != nil {
		return fmt.Errorf("could not marshal pod: %w", err)
	}

	_, err = f.Write(podYaml)

	if err != nil {
		return fmt.Errorf("could not write to file '%s': %w", yamlPath, err)
	}

	return nil
}

func (controller *Controller) dumpPodLogs(pod *apicorev1.Pod) error {
	logrus.Infof("dumping logs for pod '%s/%s'", pod.Namespace, pod.Name)
	return nil
}

func (controller *Controller) podAddHandler(obj interface{}) {
	podObj, err := getObject(obj)
	if err != nil {
		logrus.Errorf("%s", err)
	}

	pod, ok := podObj.(*apicorev1.Pod)

	if !ok {
		logrus.Errorf("could not coerce object to pod")
		return
	}

	if !controller.opts.Filter.Evaluate(*pod) {
		return
	}

	controller.workQueue.AddRateLimited(NewJob(func() {
		if err := controller.dumpPodDescription(pod); err != nil {
			logrus.Errorf("could not dump pod '%s/%s': %s", pod.Namespace, pod.Name, err)
		}
	}))
}

func (controller *Controller) podUpdateHandler(_ interface{}, obj interface{}) {
	podObj, err := getObject(obj)
	if err != nil {
		logrus.Errorf("%s", err)
	}

	pod, ok := podObj.(*apicorev1.Pod)

	if !ok {
		logrus.Errorf("could not coerce object to pod")
		return
	}

	if !controller.opts.Filter.Evaluate(*pod) {
		return
	}

	controller.workQueue.AddRateLimited(NewJob(func() {
		if err := controller.dumpPodDescription(pod); err != nil {
			logrus.Errorf("could not dump pod '%s/%s': %s", pod.Namespace, pod.Name, err)
		}
	}))
}

func (controller *Controller) podDeletedHandler(obj interface{}) {
	podObj, err := getObject(obj)
	if err != nil {
		logrus.Errorf("%s", err)
	}

	pod, ok := podObj.(*apicorev1.Pod)

	if !ok {
		logrus.Errorf("could not coerce object to pod")
		return
	}

	if !controller.opts.Filter.Evaluate(*pod) {
		return
	}

	controller.workQueue.AddRateLimited(NewJob(func() {
		if err := controller.dumpPodLogs(pod); err != nil {
			logrus.Errorf("could not dump pod '%s/%s': %s", pod.Namespace, pod.Name, err)
		}
	}))
}
