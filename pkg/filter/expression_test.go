package filter

import (
	"github.com/stretchr/testify/assert"
	apiappsv1 "k8s.io/api/apps/v1"
	apibatchv1 "k8s.io/api/batch/v1"
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

	assert.False(t, expr.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-pod",
			Namespace: "non-default",
		},
	}))

	assert.False(t, expr.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-pod-postfix",
			Namespace: "default",
		},
	}))

	assert.False(t, expr.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-pod-postfix",
			Namespace: "non-default",
		},
	}))

	assert.True(t, expr.Matches(&apicorev1.Pod{
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

func TestJob(t *testing.T) {
	expr := jobExpression{
		NamePattern:      "*-job",
		NamespacePattern: "default",
	}

	assert.True(t, expr.Matches(apibatchv1.Job{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-job",
			Namespace: "default",
		},
	}))

	assert.False(t, expr.Matches(apibatchv1.Job{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-job-postfix",
			Namespace: "default",
		},
	}))

	assert.False(t, expr.Matches(apibatchv1.Job{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-job",
			Namespace: "non-default",
		},
	}))

	assert.False(t, expr.Matches(apibatchv1.Job{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-job-postfix",
			Namespace: "non-default",
		},
	}))

	assert.True(t, expr.Matches(&apibatchv1.Job{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-job",
			Namespace: "default",
		},
	}))

	assert.False(t, expr.Matches(&apibatchv1.Job{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-job-postfix",
			Namespace: "default",
		},
	}))

	assert.False(t, expr.Matches(&apibatchv1.Job{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-job",
			Namespace: "non-default",
		},
	}))

	assert.False(t, expr.Matches(&apibatchv1.Job{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-job-postfix",
			Namespace: "non-default",
		},
	}))
}

func TestReplicaset(t *testing.T) {
	expr := replicasetExpression{
		NamePattern:      "*-replicaset",
		NamespacePattern: "default",
	}

	assert.True(t, expr.Matches(apiappsv1.ReplicaSet{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-replicaset",
			Namespace: "default",
		},
	}))

	assert.False(t, expr.Matches(apiappsv1.ReplicaSet{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-replicaset-postfix",
			Namespace: "default",
		},
	}))

	assert.False(t, expr.Matches(apiappsv1.ReplicaSet{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-replicaset",
			Namespace: "non-default",
		},
	}))

	assert.False(t, expr.Matches(apiappsv1.ReplicaSet{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-replicaset-postfix",
			Namespace: "non-default",
		},
	}))

	assert.True(t, expr.Matches(&apiappsv1.ReplicaSet{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-replicaset",
			Namespace: "default",
		},
	}))

	assert.False(t, expr.Matches(&apiappsv1.ReplicaSet{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-replicaset-postfix",
			Namespace: "default",
		},
	}))

	assert.False(t, expr.Matches(&apiappsv1.ReplicaSet{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-replicaset",
			Namespace: "non-default",
		},
	}))

	assert.False(t, expr.Matches(&apiappsv1.ReplicaSet{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-replicaset-postfix",
			Namespace: "non-default",
		},
	}))
}

func TestDeployment(t *testing.T) {
	expr := deploymentExpression{
		NamePattern:      "*-deployment",
		NamespacePattern: "default",
	}

	assert.True(t, expr.Matches(apiappsv1.Deployment{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-deployment",
			Namespace: "default",
		},
	}))

	assert.False(t, expr.Matches(apiappsv1.Deployment{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-deployment-postfix",
			Namespace: "default",
		},
	}))

	assert.False(t, expr.Matches(apiappsv1.Deployment{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-deployment",
			Namespace: "non-default",
		},
	}))

	assert.False(t, expr.Matches(apiappsv1.Deployment{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-deployment-postfix",
			Namespace: "non-default",
		},
	}))

	assert.True(t, expr.Matches(&apiappsv1.Deployment{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-deployment",
			Namespace: "default",
		},
	}))

	assert.False(t, expr.Matches(&apiappsv1.Deployment{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-deployment-postfix",
			Namespace: "default",
		},
	}))

	assert.False(t, expr.Matches(&apiappsv1.Deployment{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-deployment",
			Namespace: "non-default",
		},
	}))

	assert.False(t, expr.Matches(&apiappsv1.Deployment{
		ObjectMeta: apismeta.ObjectMeta{
			Name:      "some-deployment-postfix",
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

	assert.True(t, expr.Matches(apibatchv1.Job{
		ObjectMeta: apismeta.ObjectMeta{
			Namespace: "sample-ns",
		},
	}))

	assert.True(t, expr.Matches(apiappsv1.ReplicaSet{
		ObjectMeta: apismeta.ObjectMeta{
			Namespace: "sample-ns",
		},
	}))

	assert.True(t, expr.Matches(apiappsv1.Deployment{
		ObjectMeta: apismeta.ObjectMeta{
			Namespace: "sample-ns",
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

	// should match anything
	emptyExpr := labelExpression{}

	assert.True(t, emptyExpr.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Labels: map[string]string{},
		},
	}))

	assert.True(t, emptyExpr.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Labels: map[string]string{
				"anything": "anything",
			},
		},
	}))

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

	assert.True(t, simpleExpr.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Labels: map[string]string{
				simple.Key: simple.Value,
				"extra":    "extra",
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

	// missing the simple
	assert.False(t, union.Matches(apicorev1.Pod{
		ObjectMeta: apismeta.ObjectMeta{
			Labels: map[string]string{
				wcKey.Key:   wcKey.Key,
				wcValue.Key: wcValue.Value,
			},
		},
	}))
}
