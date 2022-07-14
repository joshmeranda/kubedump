package main

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"kubedump/pkg/collector"
	"kubedump/pkg/filter"
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
		panic(err.Error())
	}

	client, err := kubernetes.NewForConfig(config)

	if err != nil {
		panic(err.Error())
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
						EnvVars:  []string{"POD_DESCRIPTION_INTERVAL"},
					},
					&cli.Float64Flag{
						Name:     "pod-log-interval",
						Category: CategoryIntervals,
						Usage:    "the interval at which pod container logs are updated",
						Value:    1.0,
						EnvVars:  []string{"POD_LOG_INTERVAL"},
					},
					&cli.Float64Flag{
						Name:     "job-desc-interval",
						Category: CategoryIntervals,
						Usage:    "the interval at which job descriptions are updated",
						Value:    1.0,
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
