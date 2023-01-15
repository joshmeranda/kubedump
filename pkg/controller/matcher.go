package controller

import (
	"fmt"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubedump "kubedump/pkg"
)

type Matcher interface {
	Matches(resource kubedump.HandledResource) bool
}

func MatcherFromLabels(labels labels.Set) (Matcher, error) {
	if len(labels) == 0 {
		return nil, fmt.Errorf("received empty label set")
	}

	return &mapMatcher{
		labels: labels,
	}, nil
}

func MatcherFromPod(pod *apicorev1.Pod) (Matcher, error) {
	if len(pod.Spec.Volumes) == 0 {
		return nil, fmt.Errorf("pod has nothing to match")
	}

	return &podMatcher{
		volumes: pod.Spec.Volumes,
	}, nil
}

func MatcherFromLabelSelector(selector *apimetav1.LabelSelector) (Matcher, error) {
	s, err := apimetav1.LabelSelectorAsSelector(selector)

	if err != nil {
		return nil, fmt.Errorf("can not get Matcher from LabelSelector: %w", err)
	}

	return labelSelectorMatcher{
		inner: s,
	}, nil
}

type mapMatcher struct {
	labels labels.Set
}

func (matcher mapMatcher) Matches(resource kubedump.HandledResource) bool {
	labels := resource.GetLabels()

	for key, value := range matcher.labels {
		if labelValue, found := labels[key]; !found || labelValue != value {
			return false
		}
	}

	return true
}

type labelSelectorMatcher struct {
	inner labels.Selector
}

func (matcher labelSelectorMatcher) Matches(resource kubedump.HandledResource) bool {
	return matcher.inner.Matches(labels.Set(resource.GetLabels()))
}

type podMatcher struct {
	volumes []apicorev1.Volume
}

func (matcher podMatcher) Matches(resource kubedump.HandledResource) bool {
	switch resource.Kind {
	case "Secret", "ConfigMap":
	default:
		return false
	}

	for _, volume := range matcher.volumes {
		switch {
		case resource.Kind == "Secret" && volume.Secret != nil:
			return volume.Secret.SecretName == resource.GetName()
		case resource.Kind == "ConfigMap" && volume.ConfigMap != nil:
			return volume.ConfigMap.Name == resource.GetName()
		}
	}

	return false
}
