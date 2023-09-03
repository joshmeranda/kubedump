package filter

import (
	"github.com/IGLOU-EU/go-wildcard"
	kubedump "github.com/joshmeranda/kubedump/pkg"
)

type Expression interface {
	// Matches should return true if the given value is of the correct type, and satisfies the expression's conditions.
	Matches(resource kubedump.Resource) bool
}

type falsyExpression struct{}

func (_ falsyExpression) Matches(kubedump.Resource) bool {
	return false
}

type truthyExpression struct{}

func (_ truthyExpression) Matches(kubedump.Resource) bool {
	return true
}

type notExpression struct {
	inner Expression
}

func (expr notExpression) Matches(resource kubedump.Resource) bool {
	return !expr.inner.Matches(resource)
}

type andExpression struct {
	left  Expression
	right Expression
}

func (expr andExpression) Matches(resource kubedump.Resource) bool {
	return expr.left.Matches(resource) && expr.right.Matches(resource)
}

type orExpression struct {
	left  Expression
	right Expression
}

func (expr orExpression) Matches(resource kubedump.Resource) bool {
	return expr.left.Matches(resource) || expr.right.Matches(resource)
}

// todo: standardize capitlization for kind
type resourceExpression struct {
	kind             string
	namePattern      string
	namespacePattern string
}

func (expr resourceExpression) Matches(resource kubedump.Resource) bool {
	return expr.kind == resource.GetKind() &&
		wildcard.MatchSimple(expr.namespacePattern, resource.GetNamespace()) &&
		wildcard.MatchSimple(expr.namePattern, resource.GetName())
}

// namespaceExpression evaluates to true only if the given value has a Namespace matching the specified pattern.
type namespaceExpression struct {
	namespacePattern string
}

func (expr namespaceExpression) Matches(resource kubedump.Resource) bool {
	return wildcard.MatchSimple(expr.namespacePattern, resource.GetNamespace())
}

type labelExpression struct {
	labels map[string]string
}

func (expr labelExpression) Matches(resource kubedump.Resource) bool {
	if len(expr.labels) == 0 {
		return true
	}

	labels := resource.GetLabels()

	for key, value := range expr.labels {
		if lValue, found := labels[key]; !found || lValue != value {
			return false
		}
	}

	return true
}
