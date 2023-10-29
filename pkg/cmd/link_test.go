package kubedump

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/joshmeranda/kubedump/tests"
	cp "github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupLink(t *testing.T, dumpDir string) (func(), string, error) {
	tempDir := t.TempDir()

	opts := cp.Options{
		OnSymlink: func(string) cp.SymlinkAction {
			return cp.Shallow
		},
	}

	if err := cp.Copy(dumpDir, tempDir, opts); err != nil {
		return nil, "", fmt.Errorf("could not copy directories: %w", err)
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

	return teardown, tempDir, nil
}

func TestLink(t *testing.T) {
	teardown, dumpDir, err := setupLink(t, serviceDumpPath)
	require.NoError(t, err)
	defer teardown()

	err = LinkDump(dumpDir)
	require.NoError(t, err)

	isLink, err := isSymlink(path.Join(dumpDir, "default", "Service", "sample-service", "Pod", "sample-pod"))
	require.NoError(t, err)
	assert.True(t, isLink)

	isLink, err = isSymlink(path.Join(dumpDir, "default", "Pod", "sample-pod", "ConfigMap", "sample-configmap-volume"))
	require.NoError(t, err)
	assert.True(t, isLink)

	isLink, err = isSymlink(path.Join(dumpDir, "default", "Pod", "sample-pod", "Secret", "sample-secret-volume"))
	require.NoError(t, err)
	assert.True(t, isLink)
}
