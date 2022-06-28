package collector

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"os"
	"path"
	"time"
)

type ResourceType string

const (
	ResourcePod ResourceType = "pod"
	ResourceJob              = "job"
)

// createPathParents ensures that the parent directory for filePath exists.
func createPathParents(filePath string) error {
	dirname := path.Dir(filePath)

	if err := os.MkdirAll(dirname, 0755); err != nil {
		return err
	}

	return nil
}

// exists checks if a file exists.
func exists(filePath string) bool {
	_, err := os.Stat(filePath)

	return !os.IsNotExist(err)
}

func resourcePath(resourceType ResourceType, parent, namespace, name string) string {
	return path.Join(parent, namespace, string(resourceType), name)
}

func podDirPath(parent string, pod *corev1.Pod) string {
	return resourcePath(ResourcePod, parent, pod.Namespace, pod.Name)
}

func podLogsPath(parent string, pod *corev1.Pod, container string) string {
	return path.Join(podDirPath(parent, pod), "logs", container+".log")
}

func podYamlPath(parent string, pod *corev1.Pod) string {
	return path.Join(podDirPath(parent, pod), pod.Name+".yaml")
}

func jobDirPath(parent string, job *batchv1.Job) string {
	return resourcePath(ResourceJob, parent, job.Namespace, job.Name)
}

func jobYamlPath(parent string, job *batchv1.Job) string {
	return path.Join(jobDirPath(parent, job), job.Name+".yaml")
}

func mostRecentPodTransitionTime(conditions []corev1.PodCondition) time.Time {
	var mostRecent time.Time

	for _, condition := range conditions {
		if condition.LastTransitionTime.After(mostRecent) {
			mostRecent = condition.LastTransitionTime.Time.UTC()
		}
	}

	return mostRecent
}

func mostRecentJobTransitionTime(conditions []batchv1.JobCondition) time.Time {
	var mostRecent time.Time

	for _, condition := range conditions {
		if condition.LastTransitionTime.After(mostRecent) {
			mostRecent = condition.LastTransitionTime.Time.UTC()
		}
	}

	return mostRecent
}
