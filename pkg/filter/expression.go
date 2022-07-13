package filter

import (
	"github.com/IGLOU-EU/go-wildcard"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
)

type Expression interface {
	// Evaluate should return true if the given value is of the correct type, and satisfies the expression's conditions.
	Evaluate(v interface{}) bool
}

type falsyExpression struct{}

func (_ falsyExpression) Evaluate(_ interface{}) bool {
	return false
}

type truthyExpression struct{}

func (_ truthyExpression) Evaluate(_ interface{}) bool {
	return true
}

type notExpression struct {
	Inner Expression
}

func (expr notExpression) Evaluate(v interface{}) bool {
	return !expr.Inner.Evaluate(v)
}

type andExpression struct {
	Left  Expression
	Right Expression
}

func (expr andExpression) Evaluate(v interface{}) bool {
	return expr.Left.Evaluate(v) && expr.Right.Evaluate(v)
}

type orExpression struct {
	Left  Expression
	Right Expression
}

func (expr orExpression) Evaluate(v interface{}) bool {
	return expr.Left.Evaluate(v) || expr.Right.Evaluate(v)
}

// podExpression evaluates to true if the pod Name and Namespace match the specified patterns.
type podExpression struct {
	NamePattern      string
	NamespacePattern string
}

func (expr podExpression) Evaluate(v interface{}) bool {
	if pod, ok := v.(apicorev1.Pod); ok {
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

func (expr jobExpression) Evaluate(v interface{}) bool {
	switch v.(type) {
	case apicorev1.Pod:
		pod := v.(apicorev1.Pod)

		if jobName, ok := pod.Labels["job-name"]; ok {
			return wildcard.MatchSimple(expr.NamespacePattern, pod.Namespace) && wildcard.MatchSimple(expr.NamePattern, jobName)
		} else {
			return false
		}
	case apibatchv1.Job:
		job := v.(apibatchv1.Job)

		return wildcard.MatchSimple(expr.NamespacePattern, job.Namespace) && wildcard.MatchSimple(expr.NamePattern, job.Name)
	default:
		return false
	}
}
