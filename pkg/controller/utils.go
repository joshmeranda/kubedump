package controller

import (
	"fmt"
	"os"
	"path"

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
