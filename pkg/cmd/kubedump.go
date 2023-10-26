package kubedump

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"time"

	kubedump "github.com/joshmeranda/kubedump/pkg"
	"github.com/joshmeranda/kubedump/pkg/controller"
	"github.com/joshmeranda/kubedump/pkg/filter"
	"github.com/samber/lo"
	"github.com/urfave/cli/v2"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

const (
	// DefaultTimeFormat to use for times: YYYY-MM-DD-HH:MM:SS
	DefaultTimeFormat = "2006-01-02-15:04:05"

	LogFileName = "kubedump.log"

	FlagNameLogSyncTimeout = "log-sync-timeout"
)

var Version = ""

var flagLogSyncTimeout = cli.DurationFlag{
	Name:    FlagNameLogSyncTimeout,
	Usage:   "specify a timeout for container log syncs",
	Value:   time.Second * 2,
	EnvVars: []string{"KUBEDUMP_LOG_SYNC_TIMEOUT"},
}

func Dump(ctx *cli.Context) error {
	basePath := ctx.String("destination")

	if err := os.MkdirAll(basePath, 0755); err != nil && !os.IsExist(err) {
		return fmt.Errorf("could not create base path '%s': %w", basePath, err)
	}

	f, err := os.Create(path.Join(basePath, "kubedump.log"))
	if err != nil {
		return fmt.Errorf("could not create log file: %w", err)
	}

	out := io.MultiWriter(f, os.Stdout)
	loggerOptions := &slog.HandlerOptions{}

	if ctx.Bool("verbose") {
		loggerOptions.AddSource = true
		loggerOptions.Level = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(out, loggerOptions))

	kubedumpConfig, err := ConfigFromDefaultFile()
	if err != nil {
		if os.IsNotExist(err) {
			logger.Warn("no config found, using defaults")
			kubedumpConfig = DefaultConfig()
		} else {
			return fmt.Errorf("could not load kubedump config: %w", err)
		}
	}

	var dumpFilter filter.Expression
	if rawFilter := ctx.String("filter"); rawFilter != "" {
		if dumpFilter, err = filter.Parse(rawFilter); err != nil {
			return fmt.Errorf("could not parse filter from user '%s': %w", rawFilter, err)
		}
	} else {
		if dumpFilter, err = filter.Parse(kubedumpConfig.DefaultFilter); err != nil {
			return fmt.Errorf("could not parse filter from config '%s': %w", kubedumpConfig.DefaultFilter, err)
		}
	}

	config, err := clientcmd.BuildConfigFromFlags("", ctx.String("kubeconfig"))
	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	resources, err := kubedump.Discover(config)
	if err != nil {
		return err
	}
	resources = lo.Filter(resources, func(gvr schema.GroupVersionResource, i int) bool {
		for _, excluded := range kubedumpConfig.ExcludeResources {
			if gvr == excluded {
				return false
			}
		}

		return true
	})

	opts := controller.Options{
		BasePath:       basePath,
		ParentContext:  ctx.Context,
		Logger:         logger,
		LogSyncTimeout: ctx.Duration(FlagNameLogSyncTimeout),
		Resources:      resources,
	}

	var client kubernetes.Interface
	if client, err = kubernetes.NewForConfig(config); err != nil {
		return fmt.Errorf("could not create client for config: %w", err)
	}

	var dynamicclient dynamic.Interface
	if dynamicclient, err = dynamic.NewForConfig(config); err != nil {
		return fmt.Errorf("could not create dynamic client for config: %w", err)
	}

	c, err := controller.NewController(client, dynamicclient, opts)

	if err != nil {
		return fmt.Errorf("could not create controller: %w", err)
	}

	var nWorkers int
	if nWorkers = ctx.Int("workers"); nWorkers == 0 {
		nWorkers = kubedumpConfig.DefaultNWorkers
	}

	if err = c.Start(nWorkers, dumpFilter); err != nil {
		return fmt.Errorf("could not Start controller: %w", err)
	}

	<-ctx.Context.Done()

	if err = c.Stop(); err != nil {
		return fmt.Errorf("could not Stop controller: %w", err)
	}

	return err
}

func Filter(ctx *cli.Context) error {
	if nargs := ctx.Args().Len(); nargs != 2 {
		return fmt.Errorf("expected exactly 2 args, but received %d", nargs)
	}

	destination := ctx.String("destination")
	inPlace := ctx.Bool("in-place")

	if inPlace && destination != "" {
		return fmt.Errorf("--destination and --in-place cannot be used at the same time")
	}

	basePath, err := filepath.Abs(ctx.Args().First())
	if err != nil {
		return fmt.Errorf("failed to determine desitiatno dir: %w", err)
	}

	var rawFilter string
	if ctx.Args().Present() {
		rawFilter = ctx.Args().Get(1)
	}

	expression, err := filter.Parse(rawFilter)
	if err != nil {
		return fmt.Errorf("could not parse filter '%s': %w", rawFilter, err)
	}

	if inPlace {
		destination = os.TempDir()
	} else if destination == "" {
		destination = fmt.Sprintf("kubedump-filtered-%s.dump", time.Now().Format(DefaultTimeFormat))
	}

	loggerOptions := slog.HandlerOptions{}

	if ctx.Bool("verbose") {
		loggerOptions.AddSource = true
		loggerOptions.Level = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &loggerOptions))

	opts := filteringOptions{
		Filter:              expression,
		DestinationBasePath: destination,
		Logger:              logger,
	}

	if err := filterKubedumpDir(basePath, opts); err != nil {
		return fmt.Errorf("failed to filter kubedumper dir: %w", err)
	}

	if inPlace {
		if err := os.RemoveAll(basePath); err != nil {
			return fmt.Errorf("could not remove dump at '%s': %w", basePath, err)
		}

		if err := os.Rename(destination, basePath); err != nil {
			return fmt.Errorf("could not rename filtered dump at '%s': %w", basePath, err)
		}
	}

	return nil
}

func Discover(ctx *cli.Context) error {
	config, err := clientcmd.BuildConfigFromFlags("", ctx.String("kubeconfig"))
	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	resources, err := kubedump.Discover(config)
	if err != nil {
		return err
	}

	bytes, err := yaml.Marshal(resources)
	if err != nil {
		return fmt.Errorf("could not marshal schema: %w", err)
	}

	fmt.Println(string(bytes))

	return nil
}

func NewKubedumpApp() *cli.App {
	return &cli.App{
		Name:    "kubedump",
		Usage:   "collect k8s cluster resources and logs using a local client",
		Version: Version,
		Commands: []*cli.Command{
			{
				Name:  "dump",
				Usage: "collect cluster details to disk",
				Action: func(ctx *cli.Context) error {
					return Dump(ctx)
				},
				Flags: []cli.Flag{
					&cli.PathFlag{
						Name:    "destination",
						Usage:   "the directory path where the collected data will be stored",
						Value:   "kubedump.dump",
						Aliases: []string{"d"},
						EnvVars: []string{"KUBEDUMP_DESTINATION"},
					},
					&cli.StringFlag{
						Name:    "filter",
						Usage:   "the filter to use when collecting cluster resources",
						Aliases: []string{"f"},
						EnvVars: []string{"KUBEDUMP_FILTER"},
					},
					&cli.BoolFlag{
						Name:    "verbose",
						Usage:   "run kubedump verbosely",
						Value:   false,
						Aliases: []string{"V"},
						EnvVars: []string{"KUBEDUMP_VERBOSE"},
					},
					&cli.IntFlag{
						Name:    "workers",
						Usage:   "specify how many workers should run concurrently to process dump operations",
						Aliases: []string{"w"},
						EnvVars: []string{"KUBEUDMP_N_WORKERS"},
					},
					&flagLogSyncTimeout,
				},
			},
			{
				Name:      "filter",
				Usage:     "apply the given filter to a kubedump dump directory",
				Action:    Filter,
				ArgsUsage: "<dir> <filter>",
				Flags: []cli.Flag{
					&cli.PathFlag{
						Name:    "destination",
						Aliases: []string{"d"},
						Usage:   "the name of the resulting dump",
					},
					&cli.BoolFlag{
						Name:    "in-place",
						Usage:   "apply the filter to the given dump in-place",
						Value:   false,
						Aliases: []string{"i"},
					},
					&cli.BoolFlag{
						Name:    "verbose",
						Usage:   "run kubedump verbosely",
						Value:   false,
						Aliases: []string{"v"},
					},
				},
			},
			{
				Name:      "discover",
				Usage:     "discover the resources available on the cluster",
				Action:    Discover,
				ArgsUsage: "",
				Flags:     []cli.Flag{},
			},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "kubeconfig",
				Usage:   "path to the kubeconfig file to use when configuring the k8s client",
				Aliases: []string{"k"},
				EnvVars: []string{"KUBECONFIG"},
			},
		},
		Authors: []*cli.Author{
			{
				Name:  "Josh Meranda",
				Email: "joshmeranda@gmail.com",
			},
		},
		CustomAppHelpTemplate:  "",
		UseShortOptionHandling: true,
	}
}
