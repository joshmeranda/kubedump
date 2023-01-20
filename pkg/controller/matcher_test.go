package controller

import (
	kubedump "github.com/joshmeranda/kubedump/pkg"
	"github.com/joshmeranda/kubedump/tests"
	"github.com/stretchr/testify/assert"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func resourceWithLabels(labels map[string]string) kubedump.HandledResource {
	return tests.NewHandledResourceNoErr(&apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Labels: labels,
		},
	})
}

func TestEmptyLabelSet(t *testing.T) {
	matcher, err := MatcherFromLabels(map[string]string{})
	assert.Error(t, err)
	assert.Nil(t, matcher)
}

func TestLabelSet(t *testing.T) {
	matcher, err := MatcherFromLabels(map[string]string{"a": "b"})
	assert.NoError(t, err)

	assert.False(t, matcher.Matches(resourceWithLabels(map[string]string{})))
	assert.False(t, matcher.Matches(resourceWithLabels(map[string]string{"some-key": "some-value"})))
	assert.False(t, matcher.Matches(resourceWithLabels(map[string]string{"a": "c"})))
	assert.False(t, matcher.Matches(resourceWithLabels(map[string]string{"c": "b"})))

	assert.True(t, matcher.Matches(resourceWithLabels(map[string]string{"a": "b"})))
}

func TestPodMatcherSecret(t *testing.T) {
	handledSecret := tests.NewHandledResourceNoErr(&apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-secret",
			Namespace: tests.ResourceNamespace,
		},
	})

	anotherHandledSecret := tests.NewHandledResourceNoErr(&apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "another-sample-secret",
			Namespace: tests.ResourceNamespace,
		},
	})

	wrongNamespaceHandledSecret := tests.NewHandledResourceNoErr(&apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "wrong-namespace-sample-secret",
			Namespace: tests.ResourceNamespace + "-suffix",
		},
	})

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
	handledConfigMap := tests.NewHandledResourceNoErr(&apicorev1.ConfigMap{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-configmap",
			Namespace: tests.ResourceNamespace,
		},
	})

	anotherHandledConfigMap := tests.NewHandledResourceNoErr(&apicorev1.ConfigMap{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "another-sample-configmap",
			Namespace: tests.ResourceNamespace,
		},
	})

	wrongNamespaceHandledConfigMap := tests.NewHandledResourceNoErr(&apicorev1.ConfigMap{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "wrong-namespace-sample-configmap",
			Namespace: tests.ResourceNamespace + "-wrong",
		},
	})

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
	handledSecret := tests.NewHandledResourceNoErr(&apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: "sample-secret",
		},
	})

	anotherHandledSecret := tests.NewHandledResourceNoErr(&apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: "another-sample-secret",
		},
	})

	wrongNamespaceHandledSecret := tests.NewHandledResourceNoErr(&apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "wrong-namespace-sample-secret",
			Namespace: tests.ResourceNamespace + "-suffix",
		},
	})

	handledConfigMap := tests.NewHandledResourceNoErr(&apicorev1.ConfigMap{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: "sample-configmap",
		},
	})

	anotherHandledConfigMap := tests.NewHandledResourceNoErr(&apicorev1.ConfigMap{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: "another-sample-configmap",
		},
	})

	wrongNamespaceHandledConfigMap := tests.NewHandledResourceNoErr(&apicorev1.ConfigMap{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "wrong-namespace-sample-configmap",
			Namespace: tests.ResourceNamespace + "-wrong",
		},
	})

	var matcher, err = MatcherFromPod(&apicorev1.Pod{
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
