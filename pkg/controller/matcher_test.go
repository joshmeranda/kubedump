package controller

import (
	"testing"

	kubedump "github.com/joshmeranda/kubedump/pkg"
	"github.com/joshmeranda/kubedump/tests"
	"github.com/stretchr/testify/assert"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEmptyLabelSet(t *testing.T) {
	matcher, err := MatcherFromLabels(map[string]string{})
	assert.Error(t, err)
	assert.Nil(t, matcher)
}

func TestLabelSet(t *testing.T) {
	matcher, err := MatcherFromLabels(map[string]string{"a": "b"})
	assert.NoError(t, err)

	assert.False(t, matcher.Matches(kubedump.NewResourceBuilder().WithLabels(map[string]string{}).Build()))
	assert.False(t, matcher.Matches(kubedump.NewResourceBuilder().WithLabels(map[string]string{"some-key": "some-value"}).Build()))
	assert.False(t, matcher.Matches(kubedump.NewResourceBuilder().WithLabels(map[string]string{"a": "c"}).Build()))
	assert.False(t, matcher.Matches(kubedump.NewResourceBuilder().WithLabels(map[string]string{"c": "b"}).Build()))

	assert.True(t, matcher.Matches(kubedump.NewResourceBuilder().WithLabels(map[string]string{"a": "b"}).Build()))
}

func TestPodMatcherSecret(t *testing.T) {
	secret := kubedump.NewResourceBuilder().WithKind("Secret").WithName("sample-secret").WithNamespace(tests.ResourceNamespace).Build()

	anotherSecret := kubedump.NewResourceBuilder().WithKind("Secret").WithName("another-sample-secret").WithNamespace(tests.ResourceNamespace).Build()
	wrongNamespaceSecret := kubedump.NewResourceBuilder().WithKind("Secret").WithName("wrong-namespace-sample-secret").WithNamespace(tests.ResourceNamespace + "-suffix").Build()

	var matcher, err = MatcherFromPod(&apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Namespace: tests.ResourceNamespace,
		},
		Spec: apicorev1.PodSpec{
			Volumes: []apicorev1.Volume{
				{
					Name: "sample-configmap-volume",
					VolumeSource: apicorev1.VolumeSource{
						Secret: &apicorev1.SecretVolumeSource{
							SecretName: secret.GetName(),
						},
					},
				},
			},
		},
	})
	assert.NoError(t, err)

	assert.True(t, matcher.Matches(secret))
	assert.False(t, matcher.Matches(anotherSecret))
	assert.False(t, matcher.Matches(wrongNamespaceSecret))
}

func TestPodMatcherConfigMap(t *testing.T) {
	configMap := kubedump.NewResourceBuilder().WithKind("ConfigMap").WithName("sample-configmap").WithNamespace(tests.ResourceNamespace).Build()
	anotherConfigMap := kubedump.NewResourceBuilder().WithKind("ConfigMap").WithName("another-sample-configmap").WithNamespace(tests.ResourceNamespace).Build()
	wrongNamespaceConfigMap := kubedump.NewResourceBuilder().WithKind("ConfigMap").WithName("wrong-namespace-sample-configmap").WithNamespace(tests.ResourceNamespace + "-suffix").Build()

	var matcher, err = MatcherFromPod(&apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Namespace: tests.ResourceNamespace,
		},
		Spec: apicorev1.PodSpec{
			Volumes: []apicorev1.Volume{
				{
					Name: "sample-configmap-volume",
					VolumeSource: apicorev1.VolumeSource{
						ConfigMap: &apicorev1.ConfigMapVolumeSource{
							LocalObjectReference: apicorev1.LocalObjectReference{
								Name: configMap.GetName(),
							},
						},
					},
				},
			},
		},
	})
	assert.NoError(t, err)

	assert.True(t, matcher.Matches(configMap))
	assert.False(t, matcher.Matches(anotherConfigMap))
	assert.False(t, matcher.Matches(wrongNamespaceConfigMap))
}

func TestPodMatcherMixed(t *testing.T) {
	secret := kubedump.NewResourceBuilder().WithKind("Secret").WithName("sample-secret").WithNamespace(tests.ResourceNamespace).Build()
	anotherSecret := kubedump.NewResourceBuilder().WithKind("Secret").WithName("another-sample-secret").WithNamespace(tests.ResourceNamespace).Build()
	wrongNamespaceSecret := kubedump.NewResourceBuilder().WithKind("Secret").WithName("wrong-namespace-sample-secret").WithNamespace(tests.ResourceNamespace + "-suffix").Build()

	ConfigMap := kubedump.NewResourceBuilder().WithKind("ConfigMap").WithName("sample-configmap").WithNamespace(tests.ResourceNamespace).Build()
	anotherConfigMap := kubedump.NewResourceBuilder().WithKind("ConfigMap").WithName("another-sample-configmap").WithNamespace(tests.ResourceNamespace).Build()
	wrongNamespaceConfigMap := kubedump.NewResourceBuilder().WithKind("ConfigMap").WithName("wrong-namespace-sample-configmap").WithNamespace(tests.ResourceNamespace + "-suffix").Build()

	var matcher, err = MatcherFromPod(&apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Namespace: tests.ResourceNamespace,
		},
		Spec: apicorev1.PodSpec{
			Volumes: []apicorev1.Volume{
				{
					Name: "sample-configmap-volume",
					VolumeSource: apicorev1.VolumeSource{
						Secret: &apicorev1.SecretVolumeSource{
							SecretName: secret.GetName(),
						},
					},
				},
				{
					Name: "sample-configmap-volume",
					VolumeSource: apicorev1.VolumeSource{
						ConfigMap: &apicorev1.ConfigMapVolumeSource{
							LocalObjectReference: apicorev1.LocalObjectReference{
								Name: ConfigMap.GetName(),
							},
						},
					},
				},
			},
		},
	})
	assert.NoError(t, err)

	assert.True(t, matcher.Matches(secret))
	assert.False(t, matcher.Matches(anotherSecret))
	assert.False(t, matcher.Matches(wrongNamespaceSecret))

	assert.True(t, matcher.Matches(ConfigMap))
	assert.False(t, matcher.Matches(anotherConfigMap))
	assert.False(t, matcher.Matches(wrongNamespaceConfigMap))
}
