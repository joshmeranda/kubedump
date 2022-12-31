package kubedump

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	kubedump "kubedump/pkg"
	"net/http"
)

func init() {
	CmdLogger = kubedump.NewLogger()
}

func ServeKubedump(ctx *cli.Context) error {
	if ctx.Bool("verbose") {
		CmdLogger = kubedump.NewLogger(
			kubedump.WithLevel(zap.NewAtomicLevelAt(zap.DebugLevel)),
		)
	}

	opts := HandlerOptions{
		LogSyncTimeout: ctx.Duration(FlagNameLogSyncTimeout),
	}

	handler := NewHandler(CmdLogger, opts)

	CmdLogger.Infof("starting server...")

	err := http.ListenAndServe(fmt.Sprintf(":%d", kubedump.Port), &handler)

	if err != nil {
		CmdLogger.Fatal("error starting http server: %s", err)
	}

	return nil
}

func NewKubedumpServerApp() *cli.App {
	return &cli.App{
		Name:    "kubedump-server",
		Usage:   "collect k8s cluster resources and logs using a local client",
		Version: "0.2.0",
		Action:  ServeKubedump,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "verbose",
				Usage:   "run kubedump-server verbosely",
				Aliases: []string{"V"},
				EnvVars: []string{"KUBEDUMP_SERVER_DEBUG"},
			},
			&flagLogSyncTimeout,
		},
	}
}
