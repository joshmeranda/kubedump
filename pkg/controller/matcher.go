package controller

import (
	"fmt"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type LabelMatcher interface {
	Matches(labels labels.Set) bool
}

func MatcherFromLabels(labels labels.Set) (LabelMatcher, error) {
	if len(labels) == 0 {
		return nil, fmt.Errorf("received empty label set")
	}

	return &mapMatcher{
		labels: labels,
	}, nil
}

func MatcherFromLabelSelector(selector *apimetav1.LabelSelector) (LabelMatcher, error) {
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

func (matcher mapMatcher) Matches(labels labels.Set) bool {
	for key, value := range labels {
		if labelValue, found := labels[key]; !found || labelValue != value {
			return false
		}
	}

	return true
}

type labelSelectorMatcher struct {
	inner labels.Selector
}

func (matcher labelSelectorMatcher) Matches(l labels.Set) bool {
	return matcher.inner.Matches(l)
}
