package filter

import (
	"github.com/IGLOU-EU/go-wildcard"
	apicorev1 "k8s.io/api/core/v1"
)

type Expression interface {
	// Evaluate should return true if the given value is of the correct type, and satisfies the expression's conditions.
	Evaluate(v interface{}) bool
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

type podExpression struct {
	NamePattern      string
	NamespacePattern string
}

func (expr podExpression) Evaluate(v interface{}) bool {
	if pod, ok := v.(*apicorev1.Pod); ok {
		return wildcard.MatchSimple(expr.NamespacePattern, pod.Namespace) && wildcard.MatchSimple(expr.NamePattern, pod.Name)
	} else {
		return false
	}
}
