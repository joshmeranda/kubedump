package filter

import (
	"github.com/IGLOU-EU/go-wildcard"
	apicorev1 "k8s.io/api/core/v1"
)

type Expression interface {
	// Evaluate should return true if the given value is of the correct type, and satisfies the expression's conditions.
	Evaluate(v interface{}) bool
}

type NotExpression struct {
	Inner Expression
}

func (expr NotExpression) Evaluate(v interface{}) bool {
	return !expr.Inner.Evaluate(v)
}

type AndExpression struct {
	Left  Expression
	Right Expression
}

func (expr AndExpression) Evaluate(v interface{}) bool {
	return expr.Left.Evaluate(v) && expr.Right.Evaluate(v)
}

type OrExpression struct {
	Left  Expression
	Right Expression
}

func (expr OrExpression) Evaluate(v interface{}) bool {
	return expr.Left.Evaluate(v) || expr.Right.Evaluate(v)
}

type PodExpression struct {
	NamePattern      string
	NamespacePattern string
}

func (expr PodExpression) Evaluate(v interface{}) bool {
	if pod, ok := v.(*apicorev1.Pod); ok {
		return wildcard.MatchSimple(expr.NamespacePattern, pod.Namespace) && wildcard.MatchSimple(expr.NamePattern, pod.Name)
	} else {
		return false
	}
}
