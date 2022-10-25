package tests

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

	if _, err := os.Stat(destination); err == nil {
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
