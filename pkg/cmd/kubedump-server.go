package kubedump

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	kubedump "kubedump/pkg"
	"net/http"
)

func ServeKubedump(ctx *cli.Context) error {
	loggerOptions := []kubedump.LoggerOption{
		kubedump.WithPaths(BasePath),
	}

	if ctx.Bool("verbose") {
		loggerOptions = append(loggerOptions, kubedump.WithLevel(zap.NewAtomicLevelAt(zap.DebugLevel)))
	}

	logger := kubedump.NewLogger(loggerOptions...)

	opts := HandlerOptions{
		LogSyncTimeout: ctx.Duration(FlagNameLogSyncTimeout),
		Logger:         logger,
	}

	handler := NewHandler(opts)

	logger.Infof("starting server...")

	err := http.ListenAndServe(fmt.Sprintf(":%d", kubedump.Port), &handler)

	if err != nil {
		logger.Fatal("error starting http server: %s", err)
	}

	return nil
}

func NewKubedumpServerApp() *cli.App {
	return &cli.App{
		Name:    "kubedump-server",
		Usage:   "collect k8s cluster resources and logs using a local client",
		Version: Version,
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
