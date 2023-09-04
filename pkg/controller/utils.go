package controller

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	kubedump "github.com/joshmeranda/kubedump/pkg"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func getSymlinkPaths(basePath string, parent kubedump.Resource, child kubedump.Resource) (string, string, error) {
	builder := kubedump.NewResourcePathBuilder().WithBase(basePath)

	resourceBasePath := builder.WithResource(parent).Build()
	childPath := builder.WithResource(child).Build()

	linkDir := path.Join(resourceBasePath, child.GetKind())

	relPath, err := filepath.Rel(linkDir, childPath)
	if err != nil {
		return "", "", fmt.Errorf("could not get basepath for matched and matcher: %w", err)
	}

	symlinkPath := path.Join(linkDir, child.GetName())

	return symlinkPath, relPath, nil
}

func linkResource(basePath string, matcher kubedump.Resource, matched kubedump.Resource) error {
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

func dumpResourceDescription(filePath string, u *unstructured.Unstructured) error {
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

	data, err := yaml.Marshal(u)
	if err != nil {
		return fmt.Errorf("could not marshal %s: %w", u.GetKind(), err)
	}

	_, err = f.Write(data)
	if err != nil {
		return fmt.Errorf("could not write %s to file '%s': %w", u.GetKind(), filePath, err)
	}

	return nil
}

func selectorFromUnstructured(u *unstructured.Unstructured) (Matcher, error) {
	var labelSelectorPath []string
	switch u.GetKind() {
	case "Deployment", "ReplicaSet", "StatefulSet", "DaemonSet", "Job", "CronJob":
		labelSelectorPath = []string{"spec", "selector", "matchLabels"}
	case "Service":
		labelSelectorPath = []string{"spec", "selector"}
	default:
		return nil, fmt.Errorf("cannot generate a label mathcer for kind '%s'", u.GetKind())
	}

	selectors, ok, err := unstructured.NestedStringMap(u.Object, labelSelectorPath...)
	if !ok {
		return nil, fmt.Errorf("counld not find selector in unstructured object")
	}
	if err != nil {
		return nil, fmt.Errorf("colud not get selector from unstructured object: %w", err)
	}

	return MatcherFromLabelSelector(&v1.LabelSelector{
		MatchLabels: selectors,
	})
}
