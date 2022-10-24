package filter

import (
	"github.com/IGLOU-EU/go-wildcard"
	apiappsv1 "k8s.io/api/apps/v1"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Expression interface {
	// Matches should return true if the given value is of the correct type, and satisfies the expression's conditions.
	Matches(v interface{}) bool
}

type falsyExpression struct{}

func (_ falsyExpression) Matches(_ interface{}) bool {
	return false
}

type truthyExpression struct{}

func (_ truthyExpression) Matches(_ interface{}) bool {
	return true
}

type notExpression struct {
	Inner Expression
}

func (expr notExpression) Matches(v interface{}) bool {
	return !expr.Inner.Matches(v)
}

type andExpression struct {
	Left  Expression
	Right Expression
}

func (expr andExpression) Matches(v interface{}) bool {
	return expr.Left.Matches(v) && expr.Right.Matches(v)
}

type orExpression struct {
	Left  Expression
	Right Expression
}

func (expr orExpression) Matches(v interface{}) bool {
	return expr.Left.Matches(v) || expr.Right.Matches(v)
}

// podExpression evaluates to true if the pod Name and Namespace match the specified patterns.
type podExpression struct {
	NamePattern      string
	NamespacePattern string
}

func (expr podExpression) Matches(v interface{}) bool {
	if pod, ok := v.(apicorev1.Pod); ok {
		return wildcard.MatchSimple(expr.NamespacePattern, pod.Namespace) && wildcard.MatchSimple(expr.NamePattern, pod.Name)
	} else if pod, ok := v.(*apicorev1.Pod); ok {
		return wildcard.MatchSimple(expr.NamespacePattern, pod.Namespace) && wildcard.MatchSimple(expr.NamePattern, pod.Name)
	} else {
		return false
	}
}

// jobExpression evaluates to true if the pod is associated with a job whose Name and Namespace match the specified patterns.
type jobExpression struct {
	NamePattern      string
	NamespacePattern string
}

func (expr jobExpression) Matches(v interface{}) bool {
	switch v.(type) {
	case apicorev1.Pod:
		pod := v.(apicorev1.Pod)

		// todo: we probably want to search through owners instead?
		if jobName, ok := pod.Labels["job-name"]; ok {
			return wildcard.MatchSimple(expr.NamespacePattern, pod.Namespace) && wildcard.MatchSimple(expr.NamePattern, jobName)
		} else {
			return false
		}
	case apibatchv1.Job:
		job := v.(apibatchv1.Job)

		return wildcard.MatchSimple(expr.NamespacePattern, job.Namespace) && wildcard.MatchSimple(expr.NamePattern, job.Name)
	case *apicorev1.Pod:
		pod := v.(*apicorev1.Pod)
		return expr.Matches(*pod)
	case *apibatchv1.Job:
		job := v.(*apibatchv1.Job)
		return expr.Matches(*job)
	default:
		return false
	}
}

type deploymentExpression struct {
	NamePattern      string
	NamespacePattern string
}

func (expr deploymentExpression) Matches(v interface{}) bool {
	switch v.(type) {
	case apiappsv1.Deployment:
		deployment := v.(apiappsv1.Deployment)
		return wildcard.MatchSimple(expr.NamespacePattern, deployment.Namespace) && wildcard.MatchSimple(expr.NamePattern, deployment.Name)
	case *apiappsv1.Deployment:
		deployment := v.(*apiappsv1.Deployment)
		return wildcard.MatchSimple(expr.NamespacePattern, deployment.Namespace) && wildcard.MatchSimple(expr.NamePattern, deployment.Name)
	default:
		return false
	}
}

// namespaceExpression evaluates to true only if the given value has a Namespace matching the specified pattern.
type namespaceExpression struct {
	NamespacePattern string
}

func (expr namespaceExpression) Matches(v interface{}) bool {
	switch v.(type) {
	case apicorev1.Pod:
		obj := v.(apicorev1.Pod)
		return expr.checkObject(&obj)
	case *apicorev1.Pod:
		obj := v.(*apicorev1.Pod)
		return expr.checkObject(obj)
	case apibatchv1.Job:
		obj := v.(apibatchv1.Job)
		return expr.checkObject(&obj)
	case *apibatchv1.Job:
		obj := v.(*apibatchv1.Job)
		return expr.checkObject(obj)
	case apiappsv1.Deployment:
		obj := v.(apiappsv1.Deployment)
		return expr.checkObject(&obj)
	case *apiappsv1.Deployment:
		obj := v.(*apiappsv1.Deployment)
		return expr.checkObject(obj)
	default:
		return false
	}
}

func (expr namespaceExpression) checkObject(obj apismetav1.Object) bool {
	if wildcard.MatchSimple(expr.NamespacePattern, obj.GetNamespace()) {
		return true
	} else {
		return false
	}
}

type labelExpression struct {
	labelPatterns map[string]string
}

func (expr labelExpression) Matches(v interface{}) bool {
	var labels map[string]string

	switch v.(type) {
	case apicorev1.Pod:
		labels = v.(apicorev1.Pod).Labels
	case *apicorev1.Pod:
		labels = v.(*apicorev1.Pod).Labels
	case apibatchv1.Job:
		labels = v.(apibatchv1.Job).Labels
	case *apibatchv1.Job:
		labels = v.(*apibatchv1.Job).Labels
	case apiappsv1.Deployment:
		labels = v.(apiappsv1.Deployment).Labels
	case *apiappsv1.Deployment:
		labels = v.(*apiappsv1.Deployment).Labels
	default:
		return false
	}

	if len(expr.labelPatterns) == 0 {
		return true
	}

patternLoop:
	for namePattern, valuePattern := range expr.labelPatterns {
		for name, value := range labels {
			if wildcard.MatchSimple(namePattern, name) && wildcard.MatchSimple(valuePattern, value) {
				continue patternLoop
			}
		}

		return false
	}

	return true
}
