package kubedump

import (
	"github.com/stretchr/testify/assert"
	"kubedump/tests"
	"os"
	"path"
	"path/filepath"
	"testing"
)

var (
	serviceDumpPath, _ = filepath.Abs(path.Join("..", "..", "tests", "resources", "Service.dump"))
)

func setupFiltering(t *testing.T, dumpDir string) (func(), string, string) {
	tempDir := t.TempDir()
	destination := path.Join(tempDir, "Filtered.dump")
	basePath := path.Join(tempDir, path.Base(dumpDir))

	if err := tests.CopyTree(dumpDir, basePath); err != nil {
		t.Fatalf("failed to copy dump dir: %s", err)
	}

	teardown := func() {
		if t.Failed() {
			dumpDir := t.Name() + ".dump"
			t.Logf("copying temp directory into '%s' for failed test", dumpDir)

			if err := os.RemoveAll(dumpDir); err != nil && !os.IsNotExist(err) {
				t.Errorf("error removing existing test dump: %s", err)
			}

			if err := tests.CopyTree(tempDir, dumpDir); err != nil {
				t.Errorf("%s", err)
			}
		}
	}

	return teardown, destination, basePath
}

func TestFilteringOnlyChildren(t *testing.T) {
	teardown, destination, basePath := setupFiltering(t, serviceDumpPath)
	defer teardown()

	stopChan := make(chan interface{})

	app := NewKubedumpApp(stopChan)

	if err := app.Run([]string{"kubedump", "filter", "--verbose", "--destination", destination, basePath, "pod default/sample-pod"}); err != nil {
		t.Fatalf("filtering failed: %s", err)
	}

	assert.DirExists(t, path.Join(destination, "default", "Pod"))
	assert.NoDirExists(t, path.Join(destination, "default", "Service"))
}

func TestFilteringParentWithChildren(t *testing.T) {
	teardown, destination, basePath := setupFiltering(t, serviceDumpPath)
	defer teardown()

	stopChan := make(chan interface{})

	app := NewKubedumpApp(stopChan)

	if err := app.Run([]string{"kubedump", "filter", "--verbose", "--destination", destination, basePath, "service default/sample-service"}); err != nil {
		t.Fatalf("filtering failed: %s", err)
	}

	assert.DirExists(t, path.Join(destination, "default", "Pod"))
	assert.DirExists(t, path.Join(destination, "default", "Service"))
}
