package controller

import (
	"context"

	"github.com/google/uuid"
)

const (
	JobNameSyncLogs           = "sync-logs"
	JobNameAddLogStream       = "add-log-stream"
	JobNameRemoveLogStream    = "remove-log-stream"
	JobNameCheckPodData       = "check-pod-data"
	JobNameDumpResourcePrefix = "dump"
)

type Job struct {
	name string
	ctx  context.Context
	fn   *func()
}

// NewJob creates a processing job and appends a UUID to the end of the given name to add uniqueness to debuging output.
func NewJob(ctx context.Context, name string, fn func()) Job {
	name += "-" + uuid.New().String()
	return Job{
		name: name,
		ctx:  ctx,
		fn:   &fn,
	}
}
