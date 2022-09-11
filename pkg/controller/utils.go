package controller

import (
	"fmt"
	"github.com/sirupsen/logrus"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path"
	"path/filepath"
	"strings"
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

func isNamespaced(resourceKind string) bool {
	switch resourceKind {
	// list created by running: kubectl api-resources --namespaced=false
	case "ComponentStatus",
		"Namespace",
		"Node",
		"PersistentVolume",
		"MutatingWebhookConfiguration",
		"ValidatingWebhookConfiguration",
		"CustomResourceDefinition",
		"APIService",
		"TokenReview",
		"SelfSubjectAccessReview",
		"SelfSubjectRulesReview",
		"SubjectAccessReview",
		"CertificateSigningRequest",
		"FlowSchema",
		"PriorityLevelConfiguration",
		"IngressClass",
		"RuntimeClass",
		"PodSecurityPolicy",
		"ClusterRoleBinding",
		"ClusterRole",
		"PriorityClass",
		"CSIDriver",
		"CSINode",
		"StorageClass", "VolumeAttachment":
		return false
	default:
		return true
	}
}

func resourceDirPath(resourceKind string, parent string, obj apismetav1.Object) string {
	if isNamespaced(resourceKind) {
		return path.Join(parent, obj.GetNamespace(), strings.ToLower(string(resourceKind)), obj.GetName())
	} else {
		return path.Join(parent, strings.ToLower(resourceKind), obj.GetName())
	}
}

func resourceFilePath(resourceKind string, parent string, obj apismetav1.Object, name string) string {
	return path.Join(resourceDirPath(resourceKind, parent, obj), name)
}

func linkToOwner(parent string, owner apismetav1.OwnerReference, resourceKind string, obj apismetav1.Object) error {
	ownerPath := resourceDirPath(owner.Kind, parent, &apismetav1.ObjectMeta{
		Name: owner.Name,

		// because of this line we can't check for `obj.Namespace == ""` in resourceDirPath
		Namespace: obj.GetNamespace(),
	})
	ownerResourcePath := path.Join(ownerPath, strings.ToLower(resourceKind))
	objPath := resourceDirPath(resourceKind, parent, obj)

	relPath, err := filepath.Rel(ownerResourcePath, objPath)

	if err != nil {
		return fmt.Errorf("could not get baseepath for owner and obj: %w", err)
	}

	if err != nil {
		return err
	}

	symlinkPath := path.Join(resourceDirPath(owner.Kind, parent, &apismetav1.ObjectMeta{
		Name: owner.Name,

		// because of this line we can't check for `obj.Namespace == ""` in resourceDirPath
		Namespace: obj.GetNamespace(),
	}), strings.ToLower(string(resourceKind)), obj.GetName())

	if err := createPathParents(symlinkPath); err != nil {
		return fmt.Errorf("unable to create parents for symlink '%s': %w", symlinkPath, err)
	}

	if err := os.Symlink(relPath, symlinkPath); err != nil {
		return fmt.Errorf("could not create symlink '%s': %w", symlinkPath, err)
	}

	return nil
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
