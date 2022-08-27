package controller

import (
	"fmt"
	"github.com/sirupsen/logrus"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
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

func resourceDirPath(resourceKind kubedump.ResourceKind, parent string, obj apismetav1.Object) string {
	return path.Join(parent, obj.GetNamespace(), string(resourceKind), obj.GetName())
}

func resourceYamlPath(resourceKind kubedump.ResourceKind, parent string, obj apismetav1.Object) string {
	return path.Join(resourceDirPath(resourceKind, parent, obj), obj.GetName()+".yaml")
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

func getObject(obj interface{}) (apismetav1.Object, error) {
	var object apismetav1.Object
	var ok bool

	if object, ok = obj.(apismetav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return nil, fmt.Errorf("error decoding object, invalid type")
		}

		object, ok = tombstone.Obj.(apismetav1.Object)
		if !ok {
			return nil, fmt.Errorf("error decoding object tombstone, invalid type")
		}
	}

	return object, nil
}
