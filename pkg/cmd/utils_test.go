package kubedump

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"testing"
)

func TestPullChartInto(t *testing.T) {
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatalf("failde to create temp dir: %s", err)
	}
	defer os.RemoveAll(dir)

	archiveFilePath, err := pullChartInto(chartReleaseUrl, dir)
	expectedArchiveFilePath := path.Join(
		dir,
		fmt.Sprintf("kubedump-server-%s.tgz", version),
	)

	assert.NoError(t, err)
	assert.FileExists(t, archiveFilePath)
	assert.Equal(t, expectedArchiveFilePath, archiveFilePath)
}

func TestPullChartIntoNoExistUrl(t *testing.T) {
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatalf("failde to create temp dir: %s", err)
	}
	defer os.RemoveAll(dir)

	archiveFilePath, err := pullChartInto("http://theres-no-way-this-url-exists/chart.tgz", dir)

	assert.Error(t, err)
	assert.Equal(t, "", archiveFilePath)
}
