package controller

import (
	"fmt"
	"github.com/joshmeranda/kubedump/pkg"
	apiappsv1 "k8s.io/api/apps/v1"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
	"os"
	"path"
	"path/filepath"
	"sigs.k8s.io/yaml"
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

func getSymlinkPaths(basePath string, parent kubedump.HandledResource, child kubedump.HandledResource) (string, string, error) {
	builder := kubedump.NewResourcePathBuilder().WithBase(basePath)

	resourceBasePath := builder.WithResource(parent).Build()
	childPath := builder.WithResource(child).Build()

	linkDir := path.Join(resourceBasePath, child.Kind)

	relPath, err := filepath.Rel(linkDir, childPath)
	if err != nil {
		return "", "", fmt.Errorf("could not get basepath for matched and matcher: %w", err)
	}

	symlinkPath := path.Join(linkDir, child.GetName())

	return symlinkPath, relPath, nil
}

func linkResource(basePath string, matcher kubedump.HandledResource, matched kubedump.HandledResource) error {
	symlinkPath, relPath, err := getSymlinkPaths(basePath, matcher, matched)
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

func dumpResourceDescription(filePath string, resource kubedump.HandledResource) error {
	if exists(filePath) {
		if err := os.Truncate(filePath, 0); err != nil {
			return fmt.Errorf("error truncating obj yaml file '%s' : %w", filePath, err)
		}
	} else {
		if err := createPathParents(filePath); err != nil {
			return fmt.Errorf("error creating parents for obj file '%s': %s", filePath, err)
		}
	}

	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("could not open file '%s': %w", filePath, err)
	}
	defer f.Close()

	data, err := yaml.Marshal(resource.Resource)
	if err != nil {
		return fmt.Errorf("could not marshal %s: %w", resource.Kind, err)
	}

	_, err = f.Write(data)
	if err != nil {
		return fmt.Errorf("could not write %s to file '%s': %w", resource.Kind, filePath, err)
	}

	return nil
}

func selectorFromHandled(handledResource kubedump.HandledResource) (Matcher, error) {
	switch resource := handledResource.Resource.(type) {
	case *apicorev1.Pod:
		return MatcherFromPod(resource)
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
