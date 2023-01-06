package kubedump

import (
	"github.com/urfave/cli/v2"
	"time"
)

var Version = ""

const FlagNameLogSyncTimeout = "log-sync-timeout"

var flagLogSyncTimeout = cli.DurationFlag{
	Name:    FlagNameLogSyncTimeout,
	Usage:   "specify a timeout for container log syncs",
	Value:   time.Second * 2,
	EnvVars: []string{"KUBEDUMP_LOG_SYNC_TIMEOUT"},
}
