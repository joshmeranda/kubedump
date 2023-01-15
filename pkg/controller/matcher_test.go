package controller

import (
	"github.com/stretchr/testify/assert"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubedump "kubedump/pkg"
	"kubedump/tests"
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
			Name: "sample-secret",
		},
	})

	anotherHandledSecret := tests.NewHandledResourceNoErr(&apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: "another-sample-secret",
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
			},
		},
	})
	assert.NoError(t, err)

	assert.True(t, matcher.Matches(handledSecret))
	assert.False(t, matcher.Matches(anotherHandledSecret))
}

func TestPodMatcherConfigMap(t *testing.T) {
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

	var matcher, err = MatcherFromPod(&apicorev1.Pod{
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

	assert.True(t, matcher.Matches(handledConfigMap))
	assert.False(t, matcher.Matches(anotherHandledConfigMap))
}
