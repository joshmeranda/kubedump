package main

import (
	"context"
	core "k8s.io/api/core/v1"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	kdump "kubedump/pkg"
	"os"
)

func main() {
	kubeconfig := os.Getenv("KUBECONFIG")

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	client, err := kubernetes.NewForConfig(config)

	if err != nil {
		panic(err.Error())
	}

	service := &core.Service{
		Spec: core.ServiceSpec{
			Selector: map[string]string{
				"app": kdump.AppName,
			},
			Ports: []core.ServicePort{
				{
					Port:     kdump.Port,
					NodePort: kdump.Port,
				},
			},
		},
	}
	pod := &core.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      kdump.AppName,
			Namespace: kdump.Namespace,
		},
		Spec: core.PodSpec{
			Volumes: nil,
			Containers: []core.Container{
				{
					Name:       kdump.AppName,
					Image:      "joshmeranda/kdump:latest",
					Command:    []string{"kdump-server"},
					Args:       []string{},
					WorkingDir: "",
					Ports: []core.ContainerPort{
						{
							ContainerPort: kdump.Port,
						},
					},
					VolumeMounts:    nil,
					ImagePullPolicy: core.PullAlways,
				},
			},
			RestartPolicy: core.RestartPolicyAlways,
		},
	}

	podClient := client.CoreV1().Pods(kdump.Namespace)
	createdPod, err := podClient.Create(context.TODO(), pod, apismeta.CreateOptions{})

	serviceClient := client.CoreV1().Services(kdump.Namespace)
	createdService, err := serviceClient.Create(context.TODO(), service, apismeta.CreateOptions{})

	if err != nil {
		panic(err.Error())
	}

	_ = createdService
	_ = createdPod
}
