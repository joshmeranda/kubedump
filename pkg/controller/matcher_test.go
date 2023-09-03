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
	handledSecret := kubedump.NewResourceBuilder().WithKind("Secret").WithName("sample-secret").WithNamespace(tests.ResourceNamespace).Build()

	anotherHandledSecret := kubedump.NewResourceBuilder().WithKind("Secret").WithName("another-sample-secret").WithNamespace(tests.ResourceNamespace).Build()
	wrongNamespaceHandledSecret := kubedump.NewResourceBuilder().WithKind("Secret").WithName("wrong-namespace-sample-secret").WithNamespace(tests.ResourceNamespace + "-suffix").Build()

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
							SecretName: handledSecret.GetName(),
						},
					},
				},
			},
		},
	})
	assert.NoError(t, err)

	assert.True(t, matcher.Matches(handledSecret))
	assert.False(t, matcher.Matches(anotherHandledSecret))
	assert.False(t, matcher.Matches(wrongNamespaceHandledSecret))
}

func TestPodMatcherConfigMap(t *testing.T) {
	handledConfigMap := kubedump.NewResourceBuilder().WithKind("ConfigMap").WithName("sample-configmap").WithNamespace(tests.ResourceNamespace).Build()
	anotherHandledConfigMap := kubedump.NewResourceBuilder().WithKind("ConfigMap").WithName("another-sample-configmap").WithNamespace(tests.ResourceNamespace).Build()
	wrongNamespaceHandledConfigMap := kubedump.NewResourceBuilder().WithKind("ConfigMap").WithName("wrong-namespace-sample-configmap").WithNamespace(tests.ResourceNamespace + "-suffix").Build()

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
								Name: handledConfigMap.GetName(),
							},
						},
					},
				},
			},
		},
	})
	assert.NoError(t, err)

	assert.True(t, matcher.Matches(handledConfigMap))
	assert.False(t, matcher.Matches(anotherHandledConfigMap))
	assert.False(t, matcher.Matches(wrongNamespaceHandledConfigMap))
}

func TestPodMatcherMixed(t *testing.T) {
	handledSecret := kubedump.NewResourceBuilder().WithKind("Secret").WithName("sample-secret").WithNamespace(tests.ResourceNamespace).Build()
	anotherHandledSecret := kubedump.NewResourceBuilder().WithKind("Secret").WithName("another-sample-secret").WithNamespace(tests.ResourceNamespace).Build()
	wrongNamespaceHandledSecret := kubedump.NewResourceBuilder().WithKind("Secret").WithName("wrong-namespace-sample-secret").WithNamespace(tests.ResourceNamespace + "-suffix").Build()

	handledConfigMap := kubedump.NewResourceBuilder().WithKind("ConfigMap").WithName("sample-configmap").WithNamespace(tests.ResourceNamespace).Build()
	anotherHandledConfigMap := kubedump.NewResourceBuilder().WithKind("ConfigMap").WithName("another-sample-configmap").WithNamespace(tests.ResourceNamespace).Build()
	wrongNamespaceHandledConfigMap := kubedump.NewResourceBuilder().WithKind("ConfigMap").WithName("wrong-namespace-sample-configmap").WithNamespace(tests.ResourceNamespace + "-suffix").Build()

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
							SecretName: handledSecret.GetName(),
						},
					},
				},
				{
					Name: "sample-configmap-volume",
					VolumeSource: apicorev1.VolumeSource{
						ConfigMap: &apicorev1.ConfigMapVolumeSource{
							LocalObjectReference: apicorev1.LocalObjectReference{
								Name: handledConfigMap.GetName(),
							},
						},
					},
				},
			},
		},
	})
	assert.NoError(t, err)

	assert.True(t, matcher.Matches(handledSecret))
	assert.False(t, matcher.Matches(anotherHandledSecret))
	assert.False(t, matcher.Matches(wrongNamespaceHandledSecret))

	assert.True(t, matcher.Matches(handledConfigMap))
	assert.False(t, matcher.Matches(anotherHandledConfigMap))
	assert.False(t, matcher.Matches(wrongNamespaceHandledConfigMap))
}
