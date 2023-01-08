package tests

import (
	apiappsv1 "k8s.io/api/apps/v1"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var SamplePodSpec = apicorev1.PodSpec{
	Containers: []apicorev1.Container{
		{
			Name:            "test-container",
			Image:           "alpine:latest",
			Command:         []string{"sh", "-c", "while :; do date '+%F %T %z'; sleep 1; done"},
			ImagePullPolicy: "",
		},
	},
	RestartPolicy: "Never",
}

var SamplePod = apicorev1.Pod{
	ObjectMeta: apimetav1.ObjectMeta{
		Name:      "test-pod",
		Namespace: ResourceNamespace,
		Labels: map[string]string{
			kubedumpTestLabelKey: kubedumpTestLabelValue,
		},
	},
	Spec: SamplePodSpec,
}

var SampleJob = apibatchv1.Job{
	ObjectMeta: apimetav1.ObjectMeta{
		Name:      "test-job",
		Namespace: ResourceNamespace,
		Labels: map[string]string{
			kubedumpTestLabelKey: kubedumpTestLabelValue,
		},
	},
	Spec: apibatchv1.JobSpec{
		Template: apicorev1.PodTemplateSpec{
			ObjectMeta: apimetav1.ObjectMeta{
				Namespace: ResourceNamespace,
			},
			Spec: SamplePodSpec,
		},
	},
}

var SampleReplicaSet = apiappsv1.ReplicaSet{
	ObjectMeta: apimetav1.ObjectMeta{
		Name:      "test-replicaset",
		Namespace: ResourceNamespace,
		Labels: map[string]string{
			kubedumpTestLabelKey: kubedumpTestLabelValue,
		},
	},
	Spec: apiappsv1.ReplicaSetSpec{
		Selector: &apimetav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "test-replicaset",
			},
			MatchExpressions: nil,
		},
		Template: apicorev1.PodTemplateSpec{
			ObjectMeta: apimetav1.ObjectMeta{
				Namespace: ResourceNamespace,
				Labels: map[string]string{
					"app": "test-replicaset",
				},
			},
			Spec: apicorev1.PodSpec{
				Containers: []apicorev1.Container{
					{
						Name:            "test-container",
						Image:           "alpine:latest",
						Command:         []string{"sh", "-c", "while :; do date '+%F %T %z'; sleep 5; done"},
						ImagePullPolicy: "",
					},
				},
			},
		},
	},
}

var SampleDeployment = apiappsv1.Deployment{
	ObjectMeta: apimetav1.ObjectMeta{
		Name:      "test-deployment",
		Namespace: ResourceNamespace,
		Labels: map[string]string{
			"app": "test-deployment",
		},
	},
	Spec: apiappsv1.DeploymentSpec{
		Selector: &apimetav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "test-deployment",
			},
			MatchExpressions: nil,
		},
		Template: apicorev1.PodTemplateSpec{
			ObjectMeta: apimetav1.ObjectMeta{
				Namespace: ResourceNamespace,
				Labels: map[string]string{
					"app": "test-deployment",
				},
			},
			Spec: apicorev1.PodSpec{
				Containers: []apicorev1.Container{
					{
						Name:            "test-container",
						Image:           "alpine:latest",
						Command:         []string{"sh", "-c", "while :; do date '+%F %T %z'; sleep 5; done"},
						ImagePullPolicy: "",
					},
				},
			},
		},
	},
}

var SampleServicePod = apicorev1.Pod{
	ObjectMeta: apimetav1.ObjectMeta{
		Name:      "test-service-pod",
		Namespace: ResourceNamespace,
		Labels: map[string]string{
			"app": "test-service",
		},
	},
	Spec: SamplePodSpec,
}

var SampleService = apicorev1.Service{
	ObjectMeta: apimetav1.ObjectMeta{
		Name:      "test-service",
		Namespace: ResourceNamespace,
		Labels: map[string]string{
			"app":                "test-service",
			kubedumpTestLabelKey: kubedumpTestLabelValue,
		},
	},
	Spec: apicorev1.ServiceSpec{
		Ports: []apicorev1.ServicePort{
			{
				Protocol: "TCP",
				Port:     80,
			},
		},
		Selector: map[string]string{
			"app": "test-service",
		},
	},
}

var SampleConfigMap = apicorev1.ConfigMap{
	ObjectMeta: apimetav1.ObjectMeta{
		Name:      "test-configmap",
		Namespace: ResourceNamespace,
	},
	Immutable: nil,
	Data: map[string]string{
		"a": "A",
		"b": "B",
		"c": "C",
	},
	BinaryData: nil,
}

var hostPathDirOrCreate = apicorev1.HostPathDirectoryOrCreate

var SamplePodWithConfigMapVolume = apicorev1.Pod{
	ObjectMeta: apimetav1.ObjectMeta{
		Name:      "test-pod-with-configmap",
		Namespace: ResourceNamespace,
	},
	Spec: apicorev1.PodSpec{
		Containers: []apicorev1.Container{
			{
				Name:            "test-container",
				Image:           "alpine:latest",
				Command:         []string{"sh", "-c", "while :; do date '+%F %T %z'; sleep 1; done"},
				ImagePullPolicy: "",
			},
		},
		RestartPolicy: "Never",
		Volumes: []apicorev1.Volume{
			{
				Name: "sample-config-map",
				VolumeSource: apicorev1.VolumeSource{
					ConfigMap: &apicorev1.ConfigMapVolumeSource{
						LocalObjectReference: apicorev1.LocalObjectReference{
							Name: SampleConfigMap.Name,
						},
						Items: []apicorev1.KeyToPath{
							{
								Key:  "a",
								Path: "a.txt",
							},
						},
					},
				},
			},
		},
	},
}

var SampleSecret = apicorev1.Secret{
	ObjectMeta: apimetav1.ObjectMeta{
		Name:      "test-secret",
		Namespace: ResourceNamespace,
	},
	StringData: map[string]string{
		"username": "gandalf",
		"password": "mellon",
	},
	Type: apicorev1.SecretTypeOpaque,
}

var SamplePodWithSecretVolume = apicorev1.Pod{
	ObjectMeta: apimetav1.ObjectMeta{
		Name:      "test-pod-with-secret",
		Namespace: ResourceNamespace,
	},
	Spec: apicorev1.PodSpec{
		Containers: []apicorev1.Container{
			{
				Name:            "test-container",
				Image:           "alpine:latest",
				Command:         []string{"sh", "-c", "while :; do date '+%F %T %z'; sleep 1; done"},
				ImagePullPolicy: "",
			},
		},
		RestartPolicy: "Never",
		Volumes: []apicorev1.Volume{
			{
				Name: "sample-config-map",
				VolumeSource: apicorev1.VolumeSource{
					Secret: &apicorev1.SecretVolumeSource{
						SecretName: SampleSecret.Name,
						Items: []apicorev1.KeyToPath{
							{
								Key:  "password",
								Path: "password.txt",
							},
							{
								Key:  "username",
								Path: "username.txt",
							},
						},
					},
				},
			},
		},
	},
}
