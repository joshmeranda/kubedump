package filter

import (
	"github.com/IGLOU-EU/go-wildcard"
	apiappsv1 "k8s.io/api/apps/v1"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// todo: this will not work recursively
func checkOwners(expr ResourceExpression, owners []apimetav1.OwnerReference, parentKind string, namespace string) bool {
	if wildcard.MatchSimple(expr.NamespacePattern(), namespace) {
		for _, owner := range owners {
			if owner.Kind == parentKind && wildcard.MatchSimple(expr.NamePattern(), owner.Name) {
				return true
			}
		}
	}

	return false
}

type Expression interface {
	// Matches should return true if the given value is of the correct type, and satisfies the expression's conditions.
	Matches(v interface{}) bool
}

type ResourceExpression interface {
	Expression

	NamePattern() string

	NamespacePattern() string
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
	inner Expression
}

func (expr notExpression) Matches(v interface{}) bool {
	return !expr.inner.Matches(v)
}

type andExpression struct {
	left  Expression
	right Expression
}

func (expr andExpression) Matches(v interface{}) bool {
	return expr.left.Matches(v) && expr.right.Matches(v)
}

type orExpression struct {
	left  Expression
	right Expression
}

func (expr orExpression) Matches(v interface{}) bool {
	return expr.left.Matches(v) || expr.right.Matches(v)
}

// podExpression evaluates to true if the pod Name and Namespace match the specified patterns.
type podExpression struct {
	namePattern      string
	namespacePattern string
}

func (expr podExpression) Matches(v interface{}) bool {
	if pod, ok := v.(apicorev1.Pod); ok {
		return wildcard.MatchSimple(expr.namespacePattern, pod.Namespace) && wildcard.MatchSimple(expr.namePattern, pod.Name)
	} else if pod, ok := v.(*apicorev1.Pod); ok {
		return wildcard.MatchSimple(expr.namespacePattern, pod.Namespace) && wildcard.MatchSimple(expr.namePattern, pod.Name)
	} else {
		return false
	}
}

func (expr podExpression) NamePattern() string {
	return expr.namePattern
}

func (expr podExpression) NamespacePattern() string {
	return expr.namespacePattern
}

// serviceExpression evaluates to true if the service Name and Namespace match the specified patterns.
type serviceExpression struct {
	namePattern      string
	namespacePattern string
}

func (expr serviceExpression) Matches(v interface{}) bool {
	switch v.(type) {
	case apicorev1.Service:
		service := v.(apicorev1.Service)
		return wildcard.MatchSimple(expr.NamespacePattern(), service.Namespace) && wildcard.MatchSimple(expr.NamePattern(), service.Name)
	case *apicorev1.Service:
		service := v.(*apicorev1.Service)
		return wildcard.MatchSimple(expr.NamespacePattern(), service.Namespace) && wildcard.MatchSimple(expr.NamePattern(), service.Name)
	default:
		return false
	}
}

func (expr serviceExpression) NamePattern() string {
	return expr.namePattern
}

func (expr serviceExpression) NamespacePattern() string {
	return expr.namespacePattern
}

// jobExpression evaluates to true if the pod is associated with a job whose Name and Namespace match the specified patterns.
type jobExpression struct {
	namePattern      string
	namespacePattern string
}

func (expr jobExpression) Matches(v interface{}) bool {
	switch v.(type) {
	case apicorev1.Pod:
		pod := v.(apicorev1.Pod)
		return checkOwners(expr, pod.OwnerReferences, "Job", pod.Namespace)
	case *apicorev1.Pod:
		pod := v.(*apicorev1.Pod)
		return checkOwners(expr, pod.OwnerReferences, "Job", pod.Namespace)
	case apibatchv1.Job:
		job := v.(apibatchv1.Job)
		return wildcard.MatchSimple(expr.namespacePattern, job.Namespace) && wildcard.MatchSimple(expr.namePattern, job.Name)
	case *apibatchv1.Job:
		job := v.(*apibatchv1.Job)
		return wildcard.MatchSimple(expr.namespacePattern, job.Namespace) && wildcard.MatchSimple(expr.namePattern, job.Name)
	default:
		return false
	}
}

func (expr jobExpression) NamePattern() string {
	return expr.namePattern
}

func (expr jobExpression) NamespacePattern() string {
	return expr.namespacePattern
}

// replicasetExpression evaluates to true if the pod is associated with a replicaset whose Name and Namespace match the specified patterns.
type replicasetExpression struct {
	namePattern      string
	namespacePattern string
}

func (expr replicasetExpression) Matches(v interface{}) bool {
	switch v.(type) {
	case apicorev1.Pod:
		pod := v.(apicorev1.Pod)
		return checkOwners(expr, pod.OwnerReferences, "ReplicaSet", pod.Namespace)
	case *apicorev1.Pod:
		pod := v.(*apicorev1.Pod)
		return checkOwners(expr, pod.OwnerReferences, "ReplicaSet", pod.Namespace)
	case apiappsv1.ReplicaSet:
		set := v.(apiappsv1.ReplicaSet)
		return wildcard.MatchSimple(expr.namespacePattern, set.Namespace) && wildcard.MatchSimple(expr.namePattern, set.Name)
	case *apiappsv1.ReplicaSet:
		set := v.(*apiappsv1.ReplicaSet)
		return wildcard.MatchSimple(expr.namespacePattern, set.Namespace) && wildcard.MatchSimple(expr.namePattern, set.Name)
	}

	return false
}

func (expr replicasetExpression) NamePattern() string {
	return expr.namePattern
}

func (expr replicasetExpression) NamespacePattern() string {
	return expr.namespacePattern
}

// deploymentExpression evaluates to true if the pod is associated with a deployment whose Name and Namespace match the specified patterns.
type deploymentExpression struct {
	namePattern      string
	namespacePattern string
}

func (expr deploymentExpression) Matches(v interface{}) bool {
	switch v.(type) {
	case apiappsv1.ReplicaSet:
		set := v.(apiappsv1.ReplicaSet)
		return checkOwners(expr, set.OwnerReferences, "Service", set.Namespace)
	case *apiappsv1.ReplicaSet:
		set := v.(*apiappsv1.ReplicaSet)
		return checkOwners(expr, set.OwnerReferences, "Service", set.Namespace)
	case apiappsv1.Deployment:
		deployment := v.(apiappsv1.Deployment)
		return wildcard.MatchSimple(expr.namespacePattern, deployment.Namespace) && wildcard.MatchSimple(expr.namePattern, deployment.Name)
	case *apiappsv1.Deployment:
		deployment := v.(*apiappsv1.Deployment)
		return wildcard.MatchSimple(expr.namespacePattern, deployment.Namespace) && wildcard.MatchSimple(expr.namePattern, deployment.Name)
	default:
		return false
	}
}

func (expr deploymentExpression) NamePattern() string {
	return expr.namePattern
}

func (expr deploymentExpression) NamespacePattern() string {
	return expr.namespacePattern
}

// namespaceExpression evaluates to true only if the given value has a Namespace matching the specified pattern.
type namespaceExpression struct {
	namespacePattern string
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
	case apiappsv1.ReplicaSet:
		obj := v.(apiappsv1.ReplicaSet)
		return expr.checkObject(&obj)
	case *apiappsv1.ReplicaSet:
		obj := v.(*apiappsv1.ReplicaSet)
		return expr.checkObject(obj)
	case apiappsv1.Deployment:
		obj := v.(apiappsv1.Deployment)
		return expr.checkObject(&obj)
	case *apiappsv1.Deployment:
		obj := v.(*apiappsv1.Deployment)
		return expr.checkObject(obj)
	case apicorev1.Service:
		obj := v.(apicorev1.Service)
		return expr.checkObject(&obj)
	case *apicorev1.Service:
		obj := v.(*apicorev1.Service)
		return expr.checkObject(obj)
	default:
		return false
	}
}

func (expr namespaceExpression) checkObject(obj apimetav1.Object) bool {
	if wildcard.MatchSimple(expr.namespacePattern, obj.GetNamespace()) {
		return true
	} else {
		return false
	}
}

type labelExpression struct {
	labels map[string]string
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
	case apiappsv1.ReplicaSet:
		labels = v.(apiappsv1.ReplicaSet).Labels
	case *apiappsv1.ReplicaSet:
		labels = v.(*apiappsv1.ReplicaSet).Labels
	case apiappsv1.Deployment:
		labels = v.(apiappsv1.Deployment).Labels
	case *apiappsv1.Deployment:
		labels = v.(*apiappsv1.Deployment).Labels
	case apicorev1.Service:
		labels = v.(apicorev1.Service).Labels
	case *apicorev1.Service:
		labels = v.(*apicorev1.Service).Labels
	default:
		return false
	}

	if len(expr.labels) == 0 {
		return true
	}

	for key, value := range expr.labels {
		if lValue, found := labels[key]; !found || lValue != value {
			return false
		}
	}

	return true
}
