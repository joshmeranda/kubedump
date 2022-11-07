package kubedump

// todo: use runtime.HandleError over logrus.Error

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"io"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	kubedump "kubedump/pkg"
	"kubedump/pkg/controller"
	"kubedump/pkg/filter"
	"net/http"
	"os"
	"time"
)

const (
	CategoryChartReference = "Chart Reference"
	CategoryChartValues    = "Chart Values"
)

func Dump(ctx *cli.Context) error {
	if ctx.Bool("verbose") {
		logrus.SetLevel(logrus.DebugLevel)
	}

	parentPath := ctx.String("destination")
	f, err := filter.Parse(ctx.String("filter"))

	if err != nil {
		return fmt.Errorf("could not parse filter: %w", err)
	}

	opts := controller.Options{
		ParentPath: parentPath,
		Filter:     f,
	}

	config, err := clientcmd.BuildConfigFromFlags("", ctx.String("kubeconfig"))

	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)

	if err != nil {
		return fmt.Errorf("could not crete client: %w", err)
	}

	c := controller.NewController(client, opts)

	if err = c.Start(5); err != nil {
		return fmt.Errorf("could not Start controller: %w", err)
	}

	time.Sleep(time.Second * 5)

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

	chartValues := make(map[string]interface{})

	if nodePort := ctx.Int("node-port"); nodePort > 0 {
		if nodePort < 30000 || nodePort > 32767 {
			return fmt.Errorf("node port value '%d' is not in the range 30000-32767")
		}

		logrus.Infof("using user provided node port '%d'", nodePort)

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
		logrus.Errorf("could not Create action config: %s", err)
	}

	installAction := action.NewInstall(actionConfig)
	installAction.Namespace = kubedump.Namespace
	installAction.ReleaseName = kubedump.ServiceName
	installAction.CreateNamespace = true // todo: we might want this to be a flag

	release, err := installAction.Run(chart, chartValues)

	if err != nil {
		return fmt.Errorf("could not install chart: %w", err)
	} else {
		logrus.Infof("installed chart '%s'", release.Name)
	}

	return nil
}

func Start(ctx *cli.Context) error {
	u, err := serviceUrl(ctx, "/Start", map[string]string{
		"filter": ctx.String("filter"),
	})

	logrus.Infof("sending request to '%s'", u.String())

	if err != nil {
		return err
	}

	httpClient := &http.Client{}
	response, err := httpClient.Get(u.String())

	if err != nil {
		return fmt.Errorf("could not Start kubedump: %w", err)
	}

	if msg := responseErrorMessage(response); msg != "" {
		return fmt.Errorf("could not Start kubedump: %s", msg)
	}

	return nil
}

func Stop(ctx *cli.Context) error {
	u, err := serviceUrl(ctx, "/Stop", nil)

	if err != nil {
		return err
	}

	httpClient := &http.Client{}
	_, err = httpClient.Get(u.String())

	if err != nil {
		return fmt.Errorf("could not Stop kubedump: %w", err)
	}

	return nil

}

func Pull(ctx *cli.Context) error {
	u, err := serviceUrl(ctx, "/tar", nil)

	if err != nil {
		return err
	}

	httpClient := &http.Client{}
	response, err := httpClient.Get(u.String())

	if err != nil {
		return fmt.Errorf("could not request tar from kubedump: %w", err)
	}
	defer response.Body.Close()

	f, err := os.Create(fmt.Sprintf("kubedump-%d.tar.gz", time.Now().UTC().Unix()))

	if err != nil {
		return fmt.Errorf("could not Create file: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(f, response.Body)

	if err != nil {
		return fmt.Errorf("could not copy response body to file: %w", err)
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

	if err := actionConfig.Init(getter, kubedump.Namespace, os.Getenv("HELM_DRIVER"), func(f string, v ...interface{}) {
	}); err != nil {
		logrus.Errorf("could not Create uninstallAction config: %s", err)
	}

	uninstallAction := action.NewUninstall(actionConfig)

	response, err := uninstallAction.Run(kubedump.HelmReleaseName)

	if err != nil {
		return fmt.Errorf("could not uninstall chart '%s': %w", kubedump.HelmReleaseName, err)
	}

	logrus.Infof("uninstalled release '%s': %s", kubedump.HelmReleaseName, response.Info)

	if err := kubeClient.CoreV1().Namespaces().Delete(context.TODO(), kubedump.Namespace, apismeta.DeleteOptions{}); err != nil {
		return fmt.Errorf("could not delete Namespace '%s': %w", kubedump.Namespace, err)
	}

	logrus.Infof("deleted Namespace '%s'", kubedump.Namespace)

	return nil
}

func NewKubedumpApp() *cli.App {
	return &cli.App{
		Name:    "kubedump",
		Usage:   "collect k8s cluster resources and logs using a local client",
		Version: "0.2.0",
		Commands: []*cli.Command{
			{
				Name:   "dump",
				Usage:  "collect cluster details to disk",
				Action: Dump,
				Flags: []cli.Flag{
					&cli.PathFlag{
						Name:    "destination",
						Usage:   "the directory path where the collected data will be stored",
						Value:   "kubedump",
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
