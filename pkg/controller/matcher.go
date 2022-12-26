package controller

import (
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	return nil, nil
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
