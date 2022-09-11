package filter

import (
	"github.com/stretchr/testify/assert"
	apicorev1 "k8s.io/api/core/v1"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestNot(t *testing.T) {
	assert.True(t, notExpression{
		Inner: falsyExpression{},
	}.Matches(0))

	assert.False(t, notExpression{
		Inner: truthyExpression{},
	}.Matches(0))
}

func TestAnd(t *testing.T) {
	assert.True(t, andExpression{
		Left:  truthyExpression{},
		Right: truthyExpression{},
	}.Matches(0))

	assert.False(t, andExpression{
		Left:  falsyExpression{},
		Right: truthyExpression{},
	}.Matches(0))

	assert.False(t, andExpression{
		Left:  truthyExpression{},
		Right: falsyExpression{},
	}.Matches(0))

	assert.False(t, andExpression{
		Left:  falsyExpression{},
		Right: falsyExpression{},
	}.Matches(0))
}

func TestOr(t *testing.T) {
	assert.True(t, orExpression{
		Left:  truthyExpression{},
		Right: truthyExpression{},
	}.Matches(0))

	assert.True(t, orExpression{
		Left:  falsyExpression{},
		Right: truthyExpression{},
	}.Matches(0))

	assert.True(t, orExpression{
		Left:  truthyExpression{},
		Right: falsyExpression{},
	}.Matches(0))

	assert.False(t, orExpression{
		Left:  falsyExpression{},
		Right: falsyExpression{},
	}.Matches(0))
}

func TestPod(t *testing.T) {
	expr := podExpression{
		NamePattern:      "*-pod",
		NamespacePattern: "default",
	}

	assert.True(t, expr.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-pod",
			Namespace: "default",
		},
	}))

	assert.False(t, expr.Matches(&apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-pod",
			Namespace: "non-default",
		},
	}))

	assert.False(t, expr.Matches(&apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-pod-postfix",
			Namespace: "default",
		},
	}))

	assert.False(t, expr.Matches(&apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-pod-postfix",
			Namespace: "non-default",
		},
	}))
}

func TestNamespace(t *testing.T) {
	expr := namespaceExpression{
		NamespacePattern: "*-ns",
	}

	assert.True(t, expr.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Namespace: "sample-ns",
		},
	}))

	assert.False(t, expr.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Namespace: "sample-namespace",
		},
	}))

	assert.False(t, expr.Matches("non-object-values"))
}
