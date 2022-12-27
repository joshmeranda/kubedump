package controller

import (
	"fmt"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type Labels map[string]string

type LabelMatcher interface {
	Matches(labels Labels) bool
}

func MatcherFromLabels(labels Labels) (LabelMatcher, error) {
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
	labels Labels
}

func (matcher mapMatcher) Matches(labels Labels) bool {
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

func (matcher labelSelectorMatcher) Matches(l Labels) bool {
	return matcher.inner.Matches(labels.Set(l))
}
