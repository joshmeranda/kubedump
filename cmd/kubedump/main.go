package main

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"io"
	"io/ioutil"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	kubedump "kubedump/pkg"
	"kubedump/pkg/collector"
	"kubedump/pkg/filter"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"
)

const (
	CategoryIntervals = "Intervals"
)

func serviceUrl(ctx *cli.Context, path string, queries map[string]string) (url.URL, error) {
	config, err := clientcmd.BuildConfigFromFlags("", ctx.String("kubeconfig"))

	if err != nil {
		return url.URL{}, fmt.Errorf("could not load config: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)

	if err != nil {
		return url.URL{}, fmt.Errorf("could not load kubeconfig: %w", err)
	}

	service, err := client.CoreV1().Services(kubedump.Namespace).Get(context.TODO(), kubedump.ServiceName, apismeta.GetOptions{})

	if err != nil {
		return url.URL{}, fmt.Errorf("could not access service '%s': %w", kubedump.ServiceName, err)
	}

	q := url.Values{}
	for k, v := range queries {
		q.Set(k, v)
	}

	serviceUrl := url.URL{
		Scheme:   "http",
		Host:     fmt.Sprintf("%s:%d", service.Spec.ClusterIP, service.Spec.Ports[0].Port),
		Path:     path,
		RawQuery: q.Encode(),
	}

	return serviceUrl, nil
}

func durationFromSeconds(s float64) time.Duration {
	return time.Duration(s * float64(time.Second) * float64(time.Millisecond))
}

func responseErrorMessage(response *http.Response) string {
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return ""
	}

	if response.Header.Get("Content-Type") != "application/json" {
		return "could not read response from server"
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "could not read response body"
	}
	defer response.Body.Close()

	var data map[string]string
	err = json.Unmarshal(body, &data)

	if err != nil {
		return fmt.Sprintf("could not parse response from server: %s", err)
	}

	return data["error"]
}

func dump(ctx *cli.Context) error {
	parentPath := ctx.String("destination")
	f, err := filter.Parse(ctx.String("filter"))

	if err != nil {
		return fmt.Errorf("could not parse f: %w", err)
	}

	opts := collector.ClusterCollectorOptions{
		ParentPath: parentPath,
		Filter:     f,
		NamespaceCollectorOptions: collector.NamespaceCollectorOptions{
			ParentPath: parentPath,
			Filter:     f,
			PodCollectorOptions: collector.PodCollectorOptions{
				ParentPath:          parentPath,
				LogInterval:         durationFromSeconds(ctx.Float64("pod-log-interval")),
				DescriptionInterval: durationFromSeconds(ctx.Float64("pod-desc-interval")),
			},
			JobCollectorOptions: collector.JobCollectorOptions{
				ParentPath:          parentPath,
				DescriptionInterval: durationFromSeconds(ctx.Float64("job-desc-interval")),
			},
		},
	}

	config, err := clientcmd.BuildConfigFromFlags("", ctx.String("kubeconfig"))

	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)

	if err != nil {
		return fmt.Errorf("could not load kubeconfig: %w", err)
	}

	clusterCollector := collector.NewClusterCollector(client, opts)

	if err := clusterCollector.Start(); err != nil {
		return fmt.Errorf("could not start collector for cluster: %s", err)
	}

	time.Sleep(time.Second * 30)

	if err := clusterCollector.Stop(); err != nil {
		return fmt.Errorf("could not stop collector for cluster: %s", err)
	}

	return nil
}

func create(ctx *cli.Context) error {
	chartPath := path.Join("artifacts", "kubedump-server-0.1.0.tgz")
	chart, err := loader.Load(chartPath)

	if err != nil {
		return fmt.Errorf("could not load char '%s': %w", chartPath, err)
	}

	config, err := clientcmd.BuildConfigFromFlags("", ctx.String("kubeconfig"))

	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	getter := &RESTClientGetter{
		namespace:  kubedump.Namespace,
		restConfig: config,
	}

	actionConfig := new(action.Configuration)

	if err := actionConfig.Init(getter, kubedump.Namespace, os.Getenv("HELM_DRIVER"), func(f string, v ...interface{}) {
	}); err != nil {
		logrus.Errorf("could not create action config: %s", err)
	}

	installAction := action.NewInstall(actionConfig)
	installAction.Namespace = kubedump.Namespace
	installAction.ReleaseName = kubedump.ServiceName
	installAction.CreateNamespace = true // todo: we might want this to be a flag

	release, err := installAction.Run(chart, nil)

	if err != nil {
		return fmt.Errorf("could not install chart: %w", err)
	} else {
		logrus.Infof("installed chart '%s'", release.Name)
	}

	return nil
}

func start(ctx *cli.Context) error {
	u, err := serviceUrl(ctx, "/start", nil)

	logrus.Infof("sending request to '%s'", u.String())

	if err != nil {
		return err
	}

	httpClient := &http.Client{}
	response, err := httpClient.Get(u.String())

	if err != nil {
		return fmt.Errorf("could not start kubedump: %w", err)
	}

	if msg := responseErrorMessage(response); msg != "" {
		return fmt.Errorf("could not start kubedump: %s", msg)
	}

	return nil
}

func stop(ctx *cli.Context) error {
	u, err := serviceUrl(ctx, "/stop", nil)

	if err != nil {
		return err
	}

	httpClient := &http.Client{}
	_, err = httpClient.Get(u.String())

	if err != nil {
		return fmt.Errorf("could not stop kubedump: %w", err)
	}

	return nil

}

func pull(ctx *cli.Context) error {
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

	f, err := os.Create(fmt.Sprintf("kubedump-%s.tar.gz", time.Now().Format(time.RFC3339)))

	if err != nil {
		return fmt.Errorf("could not create file: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(f, response.Body)

	if err != nil {
		return fmt.Errorf("could not copy response body to file: %w", err)
	}

	return nil
}

func remove(ctx *cli.Context) error {
	config, err := clientcmd.BuildConfigFromFlags("", ctx.String("kubeconfig"))

	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)

	if err != nil {
		return fmt.Errorf("could not create kubernetes client from config: %w", err)
	}

	getter := &RESTClientGetter{
		namespace:  kubedump.Namespace,
		restConfig: config,
	}

	actionConfig := new(action.Configuration)

	if err := actionConfig.Init(getter, kubedump.Namespace, os.Getenv("HELM_DRIVER"), func(f string, v ...interface{}) {
	}); err != nil {
		logrus.Errorf("could not create uninstallAction config: %s", err)
	}

	uninstallAction := action.NewUninstall(actionConfig)

	response, err := uninstallAction.Run(kubedump.HelmReleaseName)

	if err != nil {
		return fmt.Errorf("could not uninstall chart '%s': %w", kubedump.HelmReleaseName, err)
	}

	logrus.Infof("uninstalled release '%s': %s", kubedump.HelmReleaseName, response.Info)

	if err := kubeClient.CoreV1().Namespaces().Delete(context.TODO(), kubedump.Namespace, apismeta.DeleteOptions{}); err != nil {
		return fmt.Errorf("could not delete namespace '%s': %w", kubedump.Namespace, err)
	}

	logrus.Infof("deleted namespace '%s'", kubedump.Namespace)

	return nil
}

func main() {
	app := &cli.App{
		Name:    "kubedump",
		Usage:   "collect k8s cluster resources and logs using a local client",
		Version: "0.0.0",
		Commands: []*cli.Command{
			{
				Name:   "dump",
				Usage:  "collect cluster details to disk",
				Action: dump,
				Flags: []cli.Flag{
					&cli.Float64Flag{
						Name:     "pod-desc-interval",
						Category: CategoryIntervals,
						Usage:    "the interval at which pod descriptions are updated",
						Value:    kubedump.DefaultPodDescriptionInterval,
						EnvVars:  []string{"POD_DESCRIPTION_INTERVAL"},
					},
					&cli.Float64Flag{
						Name:     "pod-log-interval",
						Category: CategoryIntervals,
						Usage:    "the interval at which pod container logs are updated",
						Value:    kubedump.DefaultPodLogInterval,
						EnvVars:  []string{"POD_LOG_INTERVAL"},
					},
					&cli.Float64Flag{
						Name:     "job-desc-interval",
						Category: CategoryIntervals,
						Usage:    "the interval at which job descriptions are updated",
						Value:    kubedump.DefaultJobDescriptionInterval,
						EnvVars:  []string{"JOB_DESCRIPTION_INTERVAL"},
					},
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
						Name:    "internal",
						Usage:   "use an internal cluster config",
						EnvVars: []string{"KUBEDUMP_INTERNAL"},
					},
				},
			},
			{
				Name:   "create",
				Usage:  "create and expose a service for teh kubedump-server",
				Action: create,
			},
			{
				Name:   "start",
				Usage:  "start capturing",
				Action: start,
				Flags:  []cli.Flag{},
			},
			{
				Name:   "stop",
				Usage:  "stop capturing ",
				Action: stop,
			},
			{
				Name:   "pull",
				Usage:  "pull the captured resources as a tar archive",
				Action: pull,
			},
			{
				Name:   "remove",
				Usage:  "remove the kubedump-serve service from the cluster",
				Action: remove,
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

	if err := app.Run(os.Args); err != nil {
		fmt.Printf("Error: %s", err)
		return
	}
}
