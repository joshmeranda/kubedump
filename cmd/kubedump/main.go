package main

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	kubedump "kubedump/pkg"
	"kubedump/pkg/collector"
	"os"
	"time"
)

const (
	CategoryIntervals = "Intervals"
)

func durationFromSeconds(s float64) time.Duration {
	return time.Duration(s * float64(time.Second) * float64(time.Millisecond))
}

func dump(ctx *cli.Context) error {
	parentPath := ctx.String("destination")

	opts := collector.NamespaceCollectorOptions{
		ParentPath: parentPath,
		PodCollectorOptions: collector.PodCollectorOptions{
			ParentPath:          parentPath,
			LogInterval:         durationFromSeconds(ctx.Float64("pod-log-interval")),
			DescriptionInterval: durationFromSeconds(ctx.Float64("pod-desc-interval")),
		},
		JobCollectorOptions: collector.JobCollectorOptions{
			ParentPath:          parentPath,
			DescriptionInterval: durationFromSeconds(ctx.Float64("job-desc-interval")),
		},
	}

	config, err := clientcmd.BuildConfigFromFlags("", ctx.String("kubeconfig"))
	if err != nil {
		panic(err.Error())
	}

	client, err := kubernetes.NewForConfig(config)

	if err != nil {
		panic(err.Error())
	}

	namespaceCollector := collector.NewNamespaceCollector(kubedump.Namespace, client, opts)

	if err := namespaceCollector.Start(); err != nil {
		return fmt.Errorf("could not start collector for namespace '%s' : %s", kubedump.Namespace, err)
	}

	time.Sleep(time.Second * 60)

	if err := namespaceCollector.Stop(); err != nil {
		return fmt.Errorf("could stop start collector for namespace '%s' : %s", kubedump.Namespace, err)
	}

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
						Value:    1.0,
						EnvVars:  []string{kubedump.PodRefreshIntervalEnv},
					},
					&cli.Float64Flag{
						Name:     "pod-log-interval",
						Category: CategoryIntervals,
						Usage:    "the interval at which pod container logs are updated",
						Value:    1.0,
						EnvVars:  []string{kubedump.PodLogRefreshIntervalEnv},
					},
					&cli.Float64Flag{
						Name:     "job-desc-interval",
						Category: CategoryIntervals,
						Usage:    "the interval at which job descriptions are updated",
						Value:    1.0,
						EnvVars:  []string{kubedump.JobRefreshIntervalEnv},
					},
					&cli.PathFlag{
						Name:    "destination",
						Usage:   "the directory path where the collected data will be stored",
						Value:   "kubedump",
						EnvVars: []string{"KUBEDUMP_DESTINATION"},
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

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
		return
	}
}
