package kubedump

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/joshmeranda/kubedump/tests"
	"github.com/stretchr/testify/assert"
)

var (
	serviceDumpPath, _       = filepath.Abs(path.Join("..", "..", "tests", "resources", "Service.dump"))
	linkedServiceDumpPath, _ = filepath.Abs(path.Join("..", "..", "tests", "resources", "LinkedService.dump"))
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

func TestFilteringUnlinked(t *testing.T) {
	teardown, destination, basePath := setupFiltering(t, serviceDumpPath)
	defer teardown()

	app := NewKubedumpApp()

	if err := app.Run([]string{"kubedump", "filter", "--verbose", "--destination", destination, basePath, "Pod default/sample-pod"}); err != nil {
		t.Fatalf("filtering failed: %s", err)
	}

	assert.DirExists(t, path.Join(destination, "default", "Pod"))
	assert.NoDirExists(t, path.Join(destination, "default", "Service"))
}

func TestFilteringLinked(t *testing.T) {
	teardown, destination, basePath := setupFiltering(t, linkedServiceDumpPath)
	defer teardown()

	app := NewKubedumpApp()

	if err := app.Run([]string{"kubedump", "filter", "--verbose", "--destination", destination, basePath, "Service default/sample-service"}); err != nil {
		t.Fatalf("filtering failed: %s", err)
	}

	assert.NoDirExists(t, path.Join(destination, "default", "Pod"))
	assert.DirExists(t, path.Join(destination, "default", "Service"))
	// todo: clean up broken links
	// assert.NoFileExists(t, path.Join(destination, "default", "Service", "sample-service", "Pod", "sample-pod.yaml"))
}
