package collector

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"os"
	"path"
	"time"
)

// createPathParents ensures that the parent directory for filePath exists.
func createPathParents(filePath string) error {
	dirname := path.Dir(filePath)

	if err := os.MkdirAll(dirname, 0755); err != os.ErrExist {
		return err
	}

	return nil
}

// exists checks if a file exists.
func exists(filePath string) bool {
	_, err := os.Stat(filePath)

	return !os.IsNotExist(err)
}

func podDirPath(root string, pod *corev1.Pod) string {
	return path.Join(root, "resources", "pods", pod.Namespace, pod.Name)
}

func podLogsPath(root string, pod *corev1.Pod, container string) string {
	return path.Join(podDirPath(root, pod), "logs", container+".log")
}

func podYamlPath(root string, pod *corev1.Pod) string {
	return path.Join(podDirPath(root, pod), pod.Name) + ".yaml"
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
