package kubedump

import (
	corev1 "k8s.io/api/core/v1"
	"os"
	"path"
	"time"
)

// CreatePathParents ensures that the parent directory for filePath exists.
func CreatePathParents(filePath string) error {
	dirname := path.Dir(filePath)

	if err := os.MkdirAll(dirname, 0755); err != os.ErrExist {
		return err
	}

	return nil
}

// Exists checks if a file Exists.
func Exists(filePath string) bool {
	_, err := os.Stat(filePath)

	return !os.IsNotExist(err)
}

func PodDirPath(root string, pod *corev1.Pod) string {
	return path.Join(root, "resources", "pods", pod.Namespace, pod.Name)
}

func PodEventsPath(root string, pod *corev1.Pod) string {
	return path.Join(PodDirPath(root, pod), "events.log")
}

func PodLogsPath(root string, pod *corev1.Pod, container string) string {
	return path.Join(PodDirPath(root, pod), "logs", container+".log")
}

func PodYamlPath(root string, pod *corev1.Pod) string {
	return path.Join(PodDirPath(root, pod), pod.Name) + ".yaml"
}

func MostRecentTransitionTime(conditions []corev1.PodCondition) time.Time {
	var mostRecent time.Time

	for _, condition := range conditions {
		if condition.LastTransitionTime.After(mostRecent) {
			mostRecent = condition.LastTransitionTime.Time.UTC()
		}
	}

	return mostRecent
}
