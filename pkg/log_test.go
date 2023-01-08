package kubedump

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
	"os"
	"path"
	"testing"
	"time"
)

const logString = "It's a dangerous business, Frodo, going out of your door. You step into the Road, and if you don't" +
	" keep your feet, there is no knowing where you might be swept off to."

func encoderConfig() zapcore.EncoderConfig {
	cfg := defaultEncoderConfig

	cfg.EncodeTime = func(t time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		zapcore.RFC3339TimeEncoder(time.Time{}, encoder)
	}

	return cfg
}

func createPaths(basePath string, paths ...string) ([]string, error) {
	joinedPaths := make([]string, 0, len(paths))

	for _, p := range paths {
		joinedPath := path.Join(basePath, p)
		joinedPaths = append(joinedPaths, joinedPath)

		f, err := os.Create(joinedPath)
		if err != nil {
			return nil, fmt.Errorf("error creating file '%s': %s", p, err)
		}

		_ = f.Close()
	}

	return joinedPaths, nil
}

func TestLogWithMultiplePaths(t *testing.T) {
	basePath := t.TempDir()
	logFilesPaths, err := createPaths(basePath, "log.1.log", "log.2.log", "log.3.log")
	if err != nil {
		t.Fatalf("failed creating log files: %s", err)
	}

	logger := NewLogger(
		WithEncoderConfig(encoderConfig()),
		WithPaths(logFilesPaths...),
	)

	logger.Info(logString)

	for _, p := range logFilesPaths {
		data, err := os.ReadFile(p)
		assert.NoError(t, err)
		assert.Regexp(t, fmt.Sprintf("0001-01-01T00:00:00Z info kubedump pkg/log_test.go:[1-9][0-9]* %s%s", logString, zapcore.DefaultLineEnding), string(data))
	}
}
