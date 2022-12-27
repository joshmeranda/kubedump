package controller

import (
	"fmt"
	apiappsv1 "k8s.io/api/apps/v1"
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

func containerLogFilePath(parentPath string, pod *apicorev1.Pod, container *apicorev1.Container) string {
	return path.Join(
		resourceDirPath(parentPath, "Pod", pod),
		container.Name+".log",
	)
}

func linkMatchedResource(parent string, matcher HandledResource, matched HandledResource) error {
	matcherPath := resourceDirPath(parent, matcher.Kind, matcher.Object)
	matcherKindPath := path.Join(matcherPath, strings.ToLower(matched.Kind))

	matchedPath := resourceDirPath(parent, matched.Kind, matched)

	relPath, err := filepath.Rel(matcherKindPath, matchedPath)
	if err != nil {
		return fmt.Errorf("could not get basepath for matched and matcher: %w", err)
	}

	symlinkPath := path.Join(resourceDirPath(parent, matcher.Kind, &apimetav1.ObjectMeta{
		Name: matcher.GetName(),

		Namespace: matched.GetNamespace(),
	}), strings.ToLower(matched.Kind), matched.GetName())

	if err := createPathParents(symlinkPath); err != nil {
		return fmt.Errorf("unable to create parents for symlnk '%s': %w", symlinkPath, err)
	}

	if err := os.Symlink(relPath, symlinkPath); err != nil && !os.IsExist(err) {
		return fmt.Errorf("could not create symlink '%s': %w", symlinkPath, err)
	}

	return nil
}

func dumpResourceDescription(parentPath string, objKind string, resource HandledResource) error {
	yamlPath := resourceFilePath(parentPath, objKind, resource.Object, resource.GetName()+".yaml")

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
		return fmt.Errorf("could not marshal %s: %w", objKind, err)
	}

	_, err = f.Write(data)

	if err != nil {
		return fmt.Errorf("could not write %s to file '%s': %w", objKind, yamlPath, err)
	}

	return nil
}

func selectorFromHandled(handledResource HandledResource) (LabelMatcher, error) {
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
