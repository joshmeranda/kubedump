package tests

import (
	"fmt"
	"github.com/gobwas/glob"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	apisappsv1 "k8s.io/api/apps/v1"
	apisbatchv1 "k8s.io/api/batch/v1"
	apiscorev1 "k8s.io/api/core/v1"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"strings"
	"testing"
)

// exists checks if a file exists.
func exists(filePath string) bool {
	_, err := os.Stat(filePath)

	return !os.IsNotExist(err)
}

// displayTree will display the entire directory structure pointed to by dir.
func displayTree(t *testing.T, dir string) {
	t.Log()
	err := filepath.Walk(dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			t.Log(path)
			return nil
		})
	t.Log()

	if err != nil {
		t.Logf("error walking directory '%s': %s", dir, err)
	}
}

// copyTree will copy the target directory to the destination directory.
//
// this is not necessarily the most optimized implementation but for a testing util it'll do.
func copyTree(t *testing.T, target string, destination string) {
	target, _ = filepath.Abs(target)
	destination, _ = filepath.Abs(destination)

	if exists(destination) {
		t.Errorf("directory '%s' already exists, not copying", destination)
		return
	}

	err := filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
		newPath := filepath.Join(destination, strings.TrimPrefix(path, target))

		if info.IsDir() {
			if err = os.MkdirAll(newPath, info.Mode()); err != nil {
				t.Errorf("could not copy directory '%s': %e", path, err)
			}
		} else {
			data, err := ioutil.ReadFile(path)

			if err != nil {
				t.Errorf("could not copy file '%s': %s", path, err)
			}

			if err := ioutil.WriteFile(newPath, data, info.Mode()); err != nil {
				t.Errorf("could not write to file '%s': %s", newPath, err)
			}
		}

		return nil
	})

	if err != nil {
		t.Logf("error walking directory")
	}
}

// unmarshalFile will attempt to marshal teh file at the given path into the given object.
func unmarshalFile(fileName string, obj interface{}) error {
	data, err := ioutil.ReadFile(fileName)

	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, obj)

	return err
}

// findGlobsIn will find all top level files in the given directory that match the given pattern.
func findGlobsIn(parent string, pattern glob.Glob) ([]string, error) {
	var found []string

	children, err := os.ReadDir(parent)
	if err != nil {
		return nil, fmt.Errorf("could not read directory '%s': %w", parent, err)
	}

	for _, child := range children {
		if pattern.Match(child.Name()) {
			found = append(found, path.Join(parent, child.Name()))
		}
	}

	return found, nil
}

// assertResourceFile will assert if the expected iven object matches the file as stored to the filesystem.
func assertResourceFile(t *testing.T, kind string, fileName string, obj apismetav1.Object) {
	var fsObj apismetav1.ObjectMeta
	var err error

	switch kind {
	case "Pod":
		var pod apiscorev1.Pod
		err = unmarshalFile(fileName, &pod)
		fsObj = pod.ObjectMeta
	case "Job":
		var job apisbatchv1.Job
		err = unmarshalFile(fileName, &job)
		fsObj = job.ObjectMeta
	case "Deployment":
		var deployment apisappsv1.Deployment
		err = unmarshalFile(fileName, &deployment)
		fsObj = deployment.ObjectMeta
	default:
		t.Errorf("unsupported kind '%s' encountered", kind)
	}

	assert.NoError(t, err)

	assert.Equal(t, obj.GetName(), fsObj.GetName())
	assert.Equal(t, obj.GetNamespace(), fsObj.GetNamespace())
}

func assertLinkGlob(t *testing.T, parent string, pattern glob.Glob) {
	found, err := findGlobsIn(parent, pattern)

	assert.NoError(t, err)
	assert.Equal(t, 1, len(found))
}
