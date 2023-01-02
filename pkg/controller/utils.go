package controller

import (
	"fmt"
	apiappsv1 "k8s.io/api/apps/v1"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubedump/pkg"
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
		"StorageClass",
		"VolumeAttachment":
		return false
	default:
		return true
	}
}

// todo: might want to refactor this to take HandledResources
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

func containerLogFilePath(basePath string, pod *apicorev1.Pod, container *apicorev1.Container) string {
	return path.Join(
		resourceDirPath(basePath, "Pod", pod),
		container.Name+".log",
	)
}

func getSymlinkPaths(basePath string, parent kubedump.HandledResource, child kubedump.HandledResource) (string, string, error) {
	resourceBasePath := resourceDirPath(basePath, parent.Kind, parent)
	childPath := resourceDirPath(basePath, child.Kind, child)

	linkDir := path.Join(resourceBasePath, strings.ToLower(child.Kind))

	relPath, err := filepath.Rel(linkDir, childPath)
	if err != nil {
		return "", "", fmt.Errorf("could not get basepath for matched and matcher: %w", err)
	}

	symlinkPath := path.Join(linkDir, child.GetName())

	return symlinkPath, relPath, nil
}

func linkResource(parent string, matcher kubedump.HandledResource, matched kubedump.HandledResource) error {
	symlinkPath, relPath, err := getSymlinkPaths(parent, matcher, matched)
	if err != nil {
		return fmt.Errorf("")
	}

	if err := createPathParents(symlinkPath); err != nil {
		return fmt.Errorf("unable to create parents for symlink '%s': %w", symlinkPath, err)
	}

	if err := os.Symlink(relPath, symlinkPath); err != nil && !os.IsExist(err) {
		return fmt.Errorf("could not create symlink '%s': %w", symlinkPath, err)
	}

	return nil
}

func dumpResourceDescription(basePath string, resource kubedump.HandledResource) error {
	yamlPath := resourceFilePath(basePath, resource.Kind, resource.Object, resource.GetName()+".yaml")

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

	data, err := yaml.Marshal(resource.Resource)

	if err != nil {
		return fmt.Errorf("could not marshal %s: %w", resource.Kind, err)
	}

	_, err = f.Write(data)

	if err != nil {
		return fmt.Errorf("could not write %s to file '%s': %w", resource.Kind, yamlPath, err)
	}

	return nil
}

func selectorFromHandled(handledResource kubedump.HandledResource) (LabelMatcher, error) {
	switch resource := handledResource.Resource.(type) {
	case *apicorev1.Service:
		return MatcherFromLabels(resource.Spec.Selector)
	case *apiappsv1.Deployment:
		return MatcherFromLabelSelector(resource.Spec.Selector)
	case *apiappsv1.ReplicaSet:
		return MatcherFromLabelSelector(resource.Spec.Selector)
	case *apibatchv1.Job:
		return MatcherFromLabelSelector(resource.Spec.Selector)
	default:
		return nil, fmt.Errorf("can not create LabelMathcher from kind '%s'", handledResource.Kind)
	}
}
