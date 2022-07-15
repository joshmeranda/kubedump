package main

import (
	"context"
	"fmt"
	"github.com/urfave/cli/v2"
	"io"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"
	kubedump "kubedump/pkg"
	"kubedump/pkg/collector"
	"kubedump/pkg/filter"
	"net/http"
	"net/url"
	"os"
	"time"
)

const (
	CategoryIntervals = "Intervals"
)

func serviceUrl(ctx *cli.Context) (url.URL, error) {
	config, err := clientcmd.BuildConfigFromFlags("", ctx.String("kubeconfig"))

	if err != nil {
		return url.URL{}, fmt.Errorf("could not load config: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)

	if err != nil {
		return url.URL{}, fmt.Errorf("could not load kubeconfig: %w", err)
	}

	service, err := client.CoreV1().Services(kubedump.Namespace).Get(context.TODO(), kubedump.ServiceName, apismeta.GetOptions{})
	serviceUrl := url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", service.Spec.ClusterIP, service.Spec.Ports[0].Port),
		Path:   "/tar",
	}

	return serviceUrl, nil
}

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
	config, err := clientcmd.BuildConfigFromFlags("", ctx.String("kubeconfig"))

	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)

	if err != nil {
		return fmt.Errorf("could not load kubeconfig: %w", err)
	}

	deployments := client.AppsV1().Deployments(kubedump.Namespace)
	_, err = deployments.Create(context.TODO(), &v1.Deployment{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "kubedump-server",
			Namespace: kubedump.Namespace,
		},
		Spec: v1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Selector: &apismeta.LabelSelector{
				MatchLabels: map[string]string{
					"app": kubedump.AppName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: apismeta.ObjectMeta{
					Name:      "kubedump-server",
					Namespace: kubedump.Namespace,
					Labels: map[string]string{
						"app": kubedump.AppName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "kubedump",
							Image: "joshmeranda/kubedump-server:0.1.0-rc0-dev-3",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: kubedump.Port,
									Protocol:      "TCP",
								},
							},
							Env:            nil,
							LivenessProbe:  nil,
							ReadinessProbe: nil,
							StartupProbe:   nil,
						},
					},
				},
			},
		},
	}, apismeta.CreateOptions{})

	if err != nil {
		return fmt.Errorf("could not deploy deployment: %w", err)
	}

	services := client.CoreV1().Services(kubedump.Namespace)
	_, err = services.Create(context.TODO(), &corev1.Service{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      kubedump.ServiceName,
			Namespace: kubedump.Namespace,
			Labels:    nil,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "",
					Protocol: "TCP",
					Port:     kubedump.Port,
				},
			},
			Type: "NodePort",
			Selector: map[string]string{
				"app": kubedump.AppName,
			},
		},
	}, apismeta.CreateOptions{})

	if err != nil {
		return fmt.Errorf("could not deploy service: %w", err)
	}

	return nil
}

func start(ctx *cli.Context) error {
	u, err := serviceUrl(ctx)

	if err != nil {
		return err
	}

	u.Path = "/start"

	httpClient := &http.Client{}
	_, err = httpClient.Get(u.String())

	if err != nil {
		return fmt.Errorf("could not start kubedump: %w", err)
	}

	return nil
}

func stop(ctx *cli.Context) error {
	u, err := serviceUrl(ctx)

	if err != nil {
		return err
	}

	u.Path = "/stop"

	httpClient := &http.Client{}
	_, err = httpClient.Get(u.String())

	if err != nil {
		return fmt.Errorf("could not stop kubedump: %w", err)
	}

	return nil

}

func pull(ctx *cli.Context) error {
	u, err := serviceUrl(ctx)

	if err != nil {
		return err
	}

	u.Path = "/tar"

	httpClient := &http.Client{}
	response, err := httpClient.Get(u.String())

	if err != nil {
		return fmt.Errorf("could not request tar from kubedump: %w", err)
	}
	defer response.Body.Close()

	f, err := os.Create(fmt.Sprintf("kubedump-%s.tar", time.Now().Format(time.RFC3339)))

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

	client, err := kubernetes.NewForConfig(config)

	if err != nil {
		return fmt.Errorf("could not load kubeconfig: %w", err)
	}

	deployments := client.AppsV1().Deployments(kubedump.Namespace)
	err = deployments.Delete(context.TODO(), "kubedump-server", apismeta.DeleteOptions{})

	if err != nil {
		return fmt.Errorf("could not delete server deployment: %w", err)
	}

	services := client.CoreV1().Services(kubedump.Namespace)
	err = services.Delete(context.TODO(), kubedump.ServiceName, apismeta.DeleteOptions{})

	if err != nil {
		return fmt.Errorf("could not delete server service: %w", err)
	}

	return nil
}

func main() {
	// go:generate cat Dockerfile

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
			},
			{
				Name:   "start",
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
