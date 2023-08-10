package kubedump

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	kubedump "github.com/joshmeranda/kubedump/pkg"
	"github.com/joshmeranda/kubedump/pkg/controller"
	"github.com/joshmeranda/kubedump/pkg/filter"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	apimeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	CategoryChartReference = "Chart Reference"
	CategoryChartValues    = "Chart Values"

	// DefaultTimeFormat to use for times: YYYY-MM-DD-HH:MM:SS
	DefaultTimeFormat = "2006-01-02-15:04:05"

	LogFileName = "kubedump.log"
)

func Dump(ctx *cli.Context, stopChan chan interface{}) error {
	basePath := ctx.String("destination")

	if err := os.MkdirAll(basePath, 0755); err != nil && !os.IsExist(err) {
		return fmt.Errorf("could not create base path '%s': %w", basePath, err)
	}

	loggerOptions := []kubedump.LoggerOption{
		kubedump.WithPaths(path.Join(basePath, LogFileName)),
	}

	if ctx.Bool("verbose") {
		loggerOptions = append(loggerOptions, kubedump.WithLevel(zap.NewAtomicLevelAt(zap.DebugLevel)))
	}

	logger := kubedump.NewLogger(loggerOptions...)

	f, err := filter.Parse(ctx.String("filter"))

	if err != nil {
		return fmt.Errorf("could not parse filter: %w", err)
	}

	opts := controller.Options{
		BasePath:       basePath,
		ParentContext:  ctx.Context,
		Logger:         logger,
		LogSyncTimeout: ctx.Duration(FlagNameLogSyncTimeout),
	}

	config, err := clientcmd.BuildConfigFromFlags("", ctx.String("kubeconfig"))

	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	c, err := controller.NewController(config, opts)

	if err != nil {
		return fmt.Errorf("could not create controller: %w", err)
	}

	if err = c.Start(ctx.Int("workers"), f); err != nil {
		return fmt.Errorf("could not Start controller: %w", err)
	}

	<-stopChan

	if err = c.Stop(); err != nil {
		return fmt.Errorf("could not Stop controller: %w", err)
	}

	return err
}

func Create(ctx *cli.Context) error {
	var chartPath string
	var err error

	if rawUrl := ctx.String("chart-url"); rawUrl != "" {
		chartPath, err = pullChartInto(rawUrl, os.TempDir())
	} else if p := ctx.String("chart-path"); p != "" {
		chartPath = p
	} else {
		chartPath, err = ensureDefaultChart()
	}

	if err != nil {
		return err
	}

	logger := kubedump.NewLogger()

	chartValues := make(map[string]interface{})

	if nodePort := ctx.Int("node-port"); nodePort > 0 {
		if nodePort < 30000 || nodePort > 32767 {
			return fmt.Errorf("node port value '%d' is not in the range 30000-32767", nodePort)
		}

		logger.Infof("using user provided node port '%d'", nodePort)

		chartValues["kubedumpServer"] = map[string]interface{}{"nodePort": nodePort}
	}

	chart, err := loader.Load(chartPath)

	if err != nil {
		return fmt.Errorf("could not load chart '%s': %w", chartPath, err)
	}

	config, err := clientcmd.BuildConfigFromFlags("", ctx.String("kubeconfig"))

	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	getter := &RESTClientGetter{
		Namespace:  kubedump.Namespace,
		Restconfig: config,
	}

	actionConfig := new(action.Configuration)

	if err := actionConfig.Init(getter, kubedump.Namespace, os.Getenv("HELM_DRIVER"), func(f string, v ...interface{}) {
	}); err != nil {
		logger.Errorf("could not Create action config: %s", err)
	}

	installAction := action.NewInstall(actionConfig)
	installAction.Namespace = kubedump.Namespace
	installAction.ReleaseName = kubedump.ServiceName
	installAction.CreateNamespace = true // todo: we might want this to be a flag

	release, err := installAction.Run(chart, chartValues)

	if err != nil {
		return fmt.Errorf("could not install chart: %w", err)
	} else {
		logger.Infof("installed chart '%s'", release.Name)
	}

	return nil
}

func Start(ctx *cli.Context) error {
	u, err := serviceUrl(ctx, "/start", map[string]string{
		"filter": ctx.String("filter"),
	})

	if err != nil {
		return err
	}

	httpClient := &http.Client{}
	response, err := httpClient.Get(u.String())

	if err != nil {
		return fmt.Errorf("could not start kubedump at '%s': %w", u.String(), err)
	}

	if msg := responseErrorMessage(response); msg != "" {
		return fmt.Errorf("could not start kubedump at '%s': %s", u.String(), msg)
	}

	logger := kubedump.NewLogger()

	logger.Infof("started remote kubedump at '%s'", u.String())

	return nil
}

func Stop(ctx *cli.Context) error {
	u, err := serviceUrl(ctx, "/stop", nil)

	if err != nil {
		return err
	}

	httpClient := &http.Client{}
	_, err = httpClient.Get(u.String())

	if err != nil {
		return fmt.Errorf("could not stop kubedump at '%s': %w", u.String(), err)
	}

	logger := kubedump.NewLogger()

	logger.Infof("stopped remote kubedump at '%s'", u.String())

	return nil
}

func Pull(ctx *cli.Context) error {
	u, err := serviceUrl(ctx, "/tar", nil)

	if err != nil {
		return err
	}

	logger := kubedump.NewLogger()

	logger.Infof("pulling kubedump tar from '%s'", u.String())
	httpClient := &http.Client{}
	response, err := httpClient.Get(u.String())

	if err != nil {
		return fmt.Errorf("could not request tar from kubedump: %w", err)
	}
	defer response.Body.Close()

	switch contentType := response.Header.Get("Content-Type"); contentType {
	case "application/json":
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return fmt.Errorf("could not read respone body: %w", err)
		}

		var data map[string]string
		if err := json.Unmarshal(body, &data); err != nil {
			return fmt.Errorf("could not marshal response body: %w", err)
		}

		return fmt.Errorf("could not pull archive: %s", data["message"])
	case "application/tar":
		tarPath := fmt.Sprintf("kubedump-%d.tar.gz", time.Now().UTC().Unix())
		f, err := os.Create(tarPath)

		if err != nil {
			return fmt.Errorf("could not Create file: %w", err)
		}
		defer f.Close()

		_, err = io.Copy(f, response.Body)

		if err != nil {
			return fmt.Errorf("could not copy response body to file: %w", err)
		}
		logger.Infof("copied tar to '%s'", tarPath)
	default:
		return fmt.Errorf("unsupported Content-Type '%s'", contentType)
	}

	return nil
}

func Remove(ctx *cli.Context) error {
	config, err := clientcmd.BuildConfigFromFlags("", ctx.String("kubeconfig"))

	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)

	if err != nil {
		return fmt.Errorf("could not Create kubernetes client from config: %w", err)
	}

	getter := &RESTClientGetter{
		Namespace:  kubedump.Namespace,
		Restconfig: config,
	}

	actionConfig := new(action.Configuration)

	logger := kubedump.NewLogger()

	if err := actionConfig.Init(getter, kubedump.Namespace, os.Getenv("HELM_DRIVER"), func(f string, v ...interface{}) {
	}); err != nil {
		logger.Errorf("could not Create uninstallAction config: %s", err)
	}

	uninstallAction := action.NewUninstall(actionConfig)

	response, err := uninstallAction.Run(kubedump.HelmReleaseName)

	if err != nil {
		return fmt.Errorf("could not uninstall chart '%s': %w", kubedump.HelmReleaseName, err)
	}

	logger.Infof("uninstalled release '%s': %s", kubedump.HelmReleaseName, response.Info)

	if err := kubeClient.CoreV1().Namespaces().Delete(context.TODO(), kubedump.Namespace, apimeta.DeleteOptions{}); err != nil {
		return fmt.Errorf("could not delete Namespace '%s': %w", kubedump.Namespace, err)
	}

	logger.Infof("deleted Namespace '%s'", kubedump.Namespace)

	return nil
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

	var logger *zap.SugaredLogger
	if ctx.Bool("verbose") {
		logger = kubedump.NewLogger(kubedump.WithLevel(zap.NewAtomicLevelAt(zap.DebugLevel)))
	} else {
		logger = kubedump.NewLogger()
	}

	opts := filteringOptions{
		Filter:              expression,
		DestinationBasePath: destination,
		Logger:              logger.Named("filtering"),
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

func NewKubedumpApp(stopChan chan interface{}) *cli.App {
	return &cli.App{
		Name:    "kubedump",
		Usage:   "collect k8s cluster resources and logs using a local client",
		Version: Version,
		Commands: []*cli.Command{
			{
				Name:  "dump",
				Usage: "collect cluster details to disk",
				Action: func(ctx *cli.Context) error {
					return Dump(ctx, stopChan)
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
						Value:   "",
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
						Value:   5,
						Aliases: []string{"w"},
						EnvVars: []string{"KUBEUDMP_N_WORKERS"},
					},
					&flagLogSyncTimeout,
				},
			},
			{
				Name:   "create",
				Usage:  "Create and expose a service for the kubedump-server",
				Action: Create,
				Flags: []cli.Flag{
					&cli.PathFlag{
						Name:     "chart-path",
						Category: CategoryChartReference,
						Usage:    "the path to the local chart tar or directory",
						EnvVars:  []string{"KUBEDUMP_SERVER_CHART_PATH"},
					},
					&cli.StringFlag{
						Name:     "chart-url",
						Category: CategoryChartReference,
						Usage:    "the url of the remote chart",
						EnvVars:  []string{"KUBEDUMP_SERVER_CHAR_URL"},
					},
					&cli.IntFlag{
						Name:     "node-port",
						Category: CategoryChartValues,
						Usage:    "set the kubedumpServer.nodePort chart value to the specified value",
					},
				},
			},
			{
				Name:   "start",
				Usage:  "Start capturing",
				Action: Start,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "service-url",
						Usage:   "the url of the kubedump-server service, if not defined kubedump will attempt to find it by inspecting the service",
						EnvVars: []string{"KUBEDUMP_SERVICE_URL"},
					},
					&cli.StringFlag{
						Name:    "filter",
						Usage:   "the filter to use when collecting cluster resources",
						Value:   "",
						Aliases: []string{"f"},
						EnvVars: []string{"KUBEDUMP_FILTER"},
					},
				},
			},
			{
				Name:   "stop",
				Usage:  "Stop capturing ",
				Action: Stop,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "service-url",
						Usage:   "the url of the kubedump-server service, if not defined kubedump will attempt to find it by inspecting the service",
						EnvVars: []string{"KUBEDUMP_SERVICE_URL"},
					},
				},
			},
			{
				Name:   "pull",
				Usage:  "Pull the captured resources as a tar archive",
				Action: Pull,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "service-url",
						Usage:   "the url of the kubedump-server service, if not defined kubedump will attempt to find it by inspecting the service",
						EnvVars: []string{"KUBEDUMP_SERVICE_URL"},
					},
				},
			},
			{
				Name:   "remove",
				Usage:  "Remove the kubedump-serve service from the cluster",
				Action: Remove,
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
