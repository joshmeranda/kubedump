package controller

import (
	"fmt"
	"github.com/sirupsen/logrus"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path"
	"path/filepath"
	"sigs.k8s.io/yaml"
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
	_, err := os.Lstat(filePath)

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

func resourceDirPath(parent string, objKind string, obj apimetav1.Object) string {
	if isNamespaced(objKind) {
		return path.Join(parent, obj.GetNamespace(), strings.ToLower(objKind), obj.GetName())
	} else {
		return path.Join(parent, strings.ToLower(objKind), obj.GetName())
	}
}

func resourceFilePath(parent string, objKind string, obj apimetav1.Object, fileName string) string {
	return path.Join(resourceDirPath(parent, objKind, obj), fileName)
}

func linkToOwner(parent string, owner apimetav1.OwnerReference, objKind string, obj apimetav1.Object) error {
	ownerPath := resourceDirPath(parent, owner.Kind, &apimetav1.ObjectMeta{
		Name: owner.Name,

		// because of this line we can't check for `obj.Namespace == ""` in resourceDirPath
		Namespace: obj.GetNamespace(),
	})
	ownerResourcePath := path.Join(ownerPath, strings.ToLower(objKind))
	objPath := resourceDirPath(parent, objKind, obj)

	relPath, err := filepath.Rel(ownerResourcePath, objPath)

	if err != nil {
		return fmt.Errorf("could not get baseepath for owner and obj: %w", err)
	}

	if err != nil {
		return err
	}

	symlinkPath := path.Join(resourceDirPath(parent, owner.Kind, &apimetav1.ObjectMeta{
		Name: owner.Name,

		// because of this line we can't check for `obj.Namespace == ""` in resourceDirPath
		Namespace: obj.GetNamespace(),
	}), strings.ToLower(objKind), obj.GetName())

	if err := createPathParents(symlinkPath); err != nil {
		return fmt.Errorf("unable to create parents for symlink '%s': %w", symlinkPath, err)
	}

	if err := os.Symlink(relPath, symlinkPath); err != nil {
		return fmt.Errorf("could not create symlink '%s': %w", symlinkPath, err)
	}

	return nil
}

func linkResourceOwners(parent string, kind string, obj apimetav1.Object) {
	for _, owner := range obj.GetOwnerReferences() {
		if err := linkToOwner(parent, owner, kind, obj); err != nil {
			logrus.Errorf("could not link %s to owners '%s/%s/%s'", kind, owner.Kind, obj.GetNamespace(), owner.Name)
		}
	}
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

func dumpResourceDescription(parentPath string, objKind string, obj apimetav1.Object) error {
	yamlPath := resourceFilePath(parentPath, objKind, obj, obj.GetName()+".yaml")

	if exists(yamlPath) {
		if err := os.Truncate(yamlPath, 0); err != nil {
			return fmt.Errorf("error truncating obj yaml file '%s' : %w", yamlPath, err)
		}
	} else {
		if err := createPathParents(yamlPath); err != nil {
			return fmt.Errorf("error creating parents for obj file '%s': %s", yamlPath, err)
		}
	}

	f, err := os.OpenFile(yamlPath, os.O_WRONLY|os.O_CREATE, 0644)

	if err != nil {
		return fmt.Errorf("could not open file '%s': %w", yamlPath, err)
	}

	jobYaml, err := yaml.Marshal(obj)

	if err != nil {
		return fmt.Errorf("could not marshal %s: %w", objKind, err)
	}

	_, err = f.Write(jobYaml)

	if err != nil {
		return fmt.Errorf("could not write %s to file '%s': %w", objKind, yamlPath, err)
	}

	return nil
}
