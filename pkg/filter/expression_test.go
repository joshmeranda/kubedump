package filter

import (
	"github.com/stretchr/testify/assert"
	apicorev1 "k8s.io/api/core/v1"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

type FalsyExpression struct{}

func (_ FalsyExpression) Evaluate(_ interface{}) bool {
	return false
}

type TruthyExpression struct{}

func (_ TruthyExpression) Evaluate(_ interface{}) bool {
	return true
}

func TestNot(t *testing.T) {
	assert.True(t, NotExpression{
		Inner: FalsyExpression{},
	}.Evaluate(0))

	assert.False(t, NotExpression{
		Inner: TruthyExpression{},
	}.Evaluate(0))
}

func TestAnd(t *testing.T) {
	assert.True(t, AndExpression{
		Left:  TruthyExpression{},
		Right: TruthyExpression{},
	}.Evaluate(0))

	assert.False(t, AndExpression{
		Left:  FalsyExpression{},
		Right: TruthyExpression{},
	}.Evaluate(0))

	assert.False(t, AndExpression{
		Left:  TruthyExpression{},
		Right: FalsyExpression{},
	}.Evaluate(0))

	assert.False(t, AndExpression{
		Left:  FalsyExpression{},
		Right: FalsyExpression{},
	}.Evaluate(0))
}

func TestOr(t *testing.T) {
	assert.True(t, OrExpression{
		Left:  TruthyExpression{},
		Right: TruthyExpression{},
	}.Evaluate(0))

	assert.True(t, OrExpression{
		Left:  FalsyExpression{},
		Right: TruthyExpression{},
	}.Evaluate(0))

	assert.True(t, OrExpression{
		Left:  TruthyExpression{},
		Right: FalsyExpression{},
	}.Evaluate(0))

	assert.False(t, OrExpression{
		Left:  FalsyExpression{},
		Right: FalsyExpression{},
	}.Evaluate(0))
}

func TestPod(t *testing.T) {
	expr := PodExpression{
		NamePattern:      "*-pod",
		NamespacePattern: "default",
	}

	assert.True(t, expr.Evaluate(&apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-pod",
			Namespace: "default",
		},
	}))

	assert.False(t, expr.Evaluate(&apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-pod",
			Namespace: "non-default",
		},
	}))

	assert.False(t, expr.Evaluate(&apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-pod-postfix",
			Namespace: "default",
		},
	}))

	assert.False(t, expr.Evaluate(&apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-pod-postfix",
			Namespace: "non-default",
		},
	}))
}
