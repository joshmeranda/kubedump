package main

import (
	"context"
	core "k8s.io/api/core/v1"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"os"
)

var KDumpNamespace = "kdump"
var KDumpAppName = "kdump"
var KDumpPort int32 = 9000

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
				"app": KDumpAppName,
			},
			Ports: []core.ServicePort{
				{
					Port:     KDumpPort,
					NodePort: KDumpPort,
				},
			},
		},
	}
	pod := &core.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      KDumpAppName,
			Namespace: KDumpNamespace,
		},
		Spec: core.PodSpec{
			Volumes: nil,
			Containers: []core.Container{
				{
					Name:       KDumpAppName,
					Image:      "joshmeranda/kdump:latest",
					Command:    []string{"kdump-server"},
					Args:       []string{},
					WorkingDir: "",
					Ports: []core.ContainerPort{
						{
							ContainerPort: KDumpPort,
						},
					},
					VolumeMounts:    nil,
					ImagePullPolicy: core.PullAlways,
				},
			},
			RestartPolicy: core.RestartPolicyAlways,
		},
	}

	podClient := client.CoreV1().Pods(KDumpNamespace)
	createdPod, err := podClient.Create(context.TODO(), pod, apismeta.CreateOptions{})

	serviceClient := client.CoreV1().Services(KDumpNamespace)
	createdService, err := serviceClient.Create(context.TODO(), service, apismeta.CreateOptions{})

	if err != nil {
		panic(err.Error())
	}

	_ = createdService
	_ = createdPod
}
