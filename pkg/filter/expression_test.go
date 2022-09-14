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

func TestLabel(t *testing.T) {
	type MapPair struct {
		Key   string
		Value string
	}

	var (
		simple  = MapPair{"app", "kubedump"}
		wcKey   = MapPair{"*-wc-key", "simple-value"}
		wcValue = MapPair{"simple-key", "*-wc-value"}
	)

	simpleExpr := labelExpression{
		labelPatterns: map[string]string{
			simple.Key: simple.Value,
		},
	}

	wcKeyExpr := labelExpression{
		labelPatterns: map[string]string{
			wcKey.Key: wcKey.Value,
		},
	}
	wcValueExpr := labelExpression{
		labelPatterns: map[string]string{
			wcValue.Key: wcValue.Value,
		},
	}

	assert.True(t, simpleExpr.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Labels: map[string]string{
				simple.Key: simple.Value,
			},
		},
	}))

	assert.False(t, simpleExpr.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Labels: map[string]string{
				simple.Key: "other value",
			},
		},
	}))

	assert.False(t, simpleExpr.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Labels: map[string]string{
				"other key": simple.Value,
			},
		},
	}))

	assert.True(t, wcKeyExpr.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Labels: map[string]string{
				wcKey.Key: wcKey.Value,
			},
		},
	}))

	assert.False(t, wcKeyExpr.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Labels: map[string]string{
				wcKey.Key: "other value",
			},
		},
	}))

	assert.False(t, wcKeyExpr.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Labels: map[string]string{
				"other key": wcKey.Value,
			},
		},
	}))

	assert.True(t, wcValueExpr.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Labels: map[string]string{
				wcValue.Key: wcValue.Value,
			},
		},
	}))

	assert.False(t, wcValueExpr.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Labels: map[string]string{
				wcValue.Key: "other value",
			},
		},
	}))

	assert.False(t, wcValueExpr.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Labels: map[string]string{
				"other key": wcValue.Value,
			},
		},
	}))

	union := labelExpression{
		labelPatterns: map[string]string{
			simple.Key:  simple.Value,
			wcKey.Key:   wcKey.Value,
			wcValue.Key: wcValue.Value,
		},
	}

	// 1 1 1
	assert.True(t, union.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Labels: map[string]string{
				simple.Key:  simple.Value,
				wcKey.Key:   wcKey.Value,
				wcValue.Key: wcValue.Value,
			},
		},
	}))

	// 1 0 1 (failed value)
	assert.False(t, union.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Labels: map[string]string{
				simple.Key:  simple.Value,
				wcKey.Key:   "other value",
				wcValue.Key: wcValue.Value,
			},
		},
	}))

	// 1 0 1 (failed key)
	assert.False(t, union.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Labels: map[string]string{
				simple.Key:  simple.Value,
				"other key": wcKey.Key,
				wcValue.Key: wcValue.Value,
			},
		},
	}))
}
