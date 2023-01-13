package kubedump

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	kubedump "kubedump/pkg"
	http2 "kubedump/pkg/http"
	"os"
	"path"
)

func ServeKubedump(ctx *cli.Context) error {
	gin.SetMode(gin.ReleaseMode)

	basePath := ctx.Path("dump-dir")

	loggerOptions := []kubedump.LoggerOption{
		kubedump.WithPaths(basePath),
	}

	if ctx.Bool("verbose") {
		loggerOptions = append(loggerOptions, kubedump.WithLevel(zap.NewAtomicLevelAt(zap.DebugLevel)))
	}

	logger := kubedump.NewLogger(loggerOptions...)

	opts := http2.ServerOptions{
		LogSyncTimeout: ctx.Duration(FlagNameLogSyncTimeout),
		Logger:         logger,
		BasePath:       basePath,
		Address:        fmt.Sprintf(":%d", kubedump.Port),
		Context:        ctx.Context,
	}

	server, err := http2.NewServer(opts)
	if err != nil {
		logger.Fatalf("could not create server: %s", err)
	}

	doneChan := make(chan interface{})

	go func() {
		if err := server.Start(); err != nil {
			logger.Fatalf("failed to start server: %s", err)
		}
	}()

	<-doneChan

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
			&cli.PathFlag{
				Name:    "dump-dir",
				Usage:   "the path to the directory to use as th ebase path",
				Aliases: []string{"d"},
				EnvVars: []string{"KUBEDUMP_DUMP_DIR"},
				Value:   path.Join(string(os.PathSeparator), "var", "lib", "kubedump.dump"),
			},
			&flagLogSyncTimeout,
		},
	}
}
