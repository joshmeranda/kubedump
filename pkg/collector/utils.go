package collector

import (
	"github.com/sirupsen/logrus"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
	kubedump "kubedump/pkg"
	"os"
	"path"
	"time"
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

func resourcePath(resourceType kubedump.ResourceType, parent, namespace, name string) string {
	return path.Join(parent, namespace, string(resourceType), name)
}

func podDirPath(parent string, pod *apicorev1.Pod) string {
	return resourcePath(kubedump.ResourcePod, parent, pod.Namespace, pod.Name)
}

func podLogsPath(parent string, pod *apicorev1.Pod, container string) string {
	return path.Join(podDirPath(parent, pod), "logs", container+".log")
}

func podYamlPath(parent string, pod *apicorev1.Pod) string {
	return path.Join(podDirPath(parent, pod), pod.Name+".yaml")
}

func jobDirPath(parent string, job *apibatchv1.Job) string {
	return resourcePath(kubedump.ResourceJob, parent, job.Namespace, job.Name)
}

func jobYamlPath(parent string, job *apibatchv1.Job) string {
	return path.Join(jobDirPath(parent, job), job.Name+".yaml")
}

func mostRecentPodTransitionTime(conditions []apicorev1.PodCondition) time.Time {
	var mostRecent time.Time

	for _, condition := range conditions {
		if condition.LastTransitionTime.After(mostRecent) {
			mostRecent = condition.LastTransitionTime.Time.UTC()
		}
	}

	return mostRecent
}

func mostRecentJobTransitionTime(conditions []apibatchv1.JobCondition) time.Time {
	var mostRecent time.Time

	for _, condition := range conditions {
		if condition.LastTransitionTime.After(mostRecent) {
			mostRecent = condition.LastTransitionTime.Time.UTC()
		}
	}

	return mostRecent
}

func resourceFields(objs ...interface{}) logrus.Fields {
	fields := logrus.Fields{}

	for _, obj := range objs {
		switch obj.(type) {
		case *apicorev1.Pod:
			pod, _ := obj.(*apicorev1.Pod)

			fields["namespace"] = pod.Namespace
			fields["pod"] = pod.Name

		case *apibatchv1.Job:
			job, _ := obj.(*apibatchv1.Job)

			fields["namespace"] = job.Namespace
			fields["job"] = job.Name

		case apicorev1.Container:
			cnt, _ := obj.(apicorev1.Container)

			fields["container"] = cnt.Name

		case *apicorev1.Namespace:
			namespace, _ := obj.(*apicorev1.Namespace)

			fields["namespace"] = namespace.Name

		default:
			// uncomment when checking types
			//fields["type"] = reflect.TypeOf(obj)
		}
	}

	return fields
}
