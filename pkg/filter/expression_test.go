package filter

import (
	"github.com/stretchr/testify/assert"
	apiappsv1 "k8s.io/api/apps/v1"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestNot(t *testing.T) {
	assert.True(t, notExpression{
		inner: falsyExpression{},
	}.Matches(0))

	assert.False(t, notExpression{
		inner: truthyExpression{},
	}.Matches(0))
}

func TestAnd(t *testing.T) {
	assert.True(t, andExpression{
		left:  truthyExpression{},
		right: truthyExpression{},
	}.Matches(0))

	assert.False(t, andExpression{
		left:  falsyExpression{},
		right: truthyExpression{},
	}.Matches(0))

	assert.False(t, andExpression{
		left:  truthyExpression{},
		right: falsyExpression{},
	}.Matches(0))

	assert.False(t, andExpression{
		left:  falsyExpression{},
		right: falsyExpression{},
	}.Matches(0))
}

func TestOr(t *testing.T) {
	assert.True(t, orExpression{
		left:  truthyExpression{},
		right: truthyExpression{},
	}.Matches(0))

	assert.True(t, orExpression{
		left:  falsyExpression{},
		right: truthyExpression{},
	}.Matches(0))

	assert.True(t, orExpression{
		left:  truthyExpression{},
		right: falsyExpression{},
	}.Matches(0))

	assert.False(t, orExpression{
		left:  falsyExpression{},
		right: falsyExpression{},
	}.Matches(0))
}

func TestCheckOwners(t *testing.T) {
	type Entry struct {
		Owners      []apimetav1.OwnerReference
		ShouldMatch bool
	}

	expr := jobExpression{
		namePattern:      "*-job",
		namespacePattern: "default",
	}

	entries := []Entry{
		{
			Owners: []apimetav1.OwnerReference{
				{
					Kind: "Job",
					Name: "test-job",
				},
			},
			ShouldMatch: true,
		},
		{
			Owners: []apimetav1.OwnerReference{
				{
					Kind: "Job",
					Name: "test-job-postfix",
				},
			},
			ShouldMatch: false,
		},
	}

	for _, entry := range entries {
		if entry.ShouldMatch {
			assert.True(t, checkOwners(expr, entry.Owners, "Job", "default"))
		} else {
			assert.False(t, checkOwners(expr, entry.Owners, "Job", "default"))
		}
	}
}

func TestPod(t *testing.T) {
	type Entry struct {
		Pod         apicorev1.Pod
		ShouldMatch bool
	}

	expr := podExpression{
		namePattern:      "*-pod",
		namespacePattern: "default",
	}

	cases := []Entry{
		{
			Pod: apicorev1.Pod{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "some-pod",
					Namespace: "default",
				},
			},
			ShouldMatch: true,
		},
		{
			Pod: apicorev1.Pod{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "some-pod",
					Namespace: "non-default",
				},
			},
			ShouldMatch: false,
		},
		{
			Pod: apicorev1.Pod{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "some-pod-postfix",
					Namespace: "default",
				},
			},
			ShouldMatch: false,
		},
		{
			Pod: apicorev1.Pod{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "some-pod-postfix",
					Namespace: "non-default",
				},
			},
			ShouldMatch: false,
		},
	}

	for _, entry := range cases {
		if entry.ShouldMatch {
			assert.True(t, expr.Matches(entry.Pod))
			assert.True(t, expr.Matches(&entry.Pod))
		} else {
			assert.False(t, expr.Matches(entry.Pod))
			assert.False(t, expr.Matches(&entry.Pod))
		}
	}
}

func TestJob(t *testing.T) {
	type Entry struct {
		Job         apibatchv1.Job
		ShouldMatch bool
	}

	expr := jobExpression{
		namePattern:      "*-job",
		namespacePattern: "default",
	}

	cases := []Entry{
		{
			Job: apibatchv1.Job{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "some-job",
					Namespace: "default",
				},
			},
			ShouldMatch: true,
		},
		{
			Job: apibatchv1.Job{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "some-job",
					Namespace: "non-default",
				},
			},
			ShouldMatch: false,
		},
		{
			Job: apibatchv1.Job{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "some-job-postfix",
					Namespace: "default",
				},
			},
			ShouldMatch: false,
		},
		{
			Job: apibatchv1.Job{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "some-job-postfix",
					Namespace: "non-default",
				},
			},
			ShouldMatch: false,
		},
	}

	for _, entry := range cases {
		if entry.ShouldMatch {
			assert.True(t, expr.Matches(entry.Job))
			assert.True(t, expr.Matches(&entry.Job))
		} else {
			assert.False(t, expr.Matches(entry.Job))
			assert.False(t, expr.Matches(&entry.Job))
		}
	}
}

func TestReplicaset(t *testing.T) {
	type Entry struct {
		Set         apiappsv1.ReplicaSet
		ShouldMatch bool
	}

	expr := replicasetExpression{
		namePattern:      "*-replicaset",
		namespacePattern: "default",
	}

	cases := []Entry{
		{
			Set: apiappsv1.ReplicaSet{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "some-replicaset",
					Namespace: "default",
				},
			},
			ShouldMatch: true,
		},
		{
			Set: apiappsv1.ReplicaSet{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "some-replicaset",
					Namespace: "non-default",
				},
			},
			ShouldMatch: false,
		},
		{
			Set: apiappsv1.ReplicaSet{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "some-replicaset-postfix",
					Namespace: "default",
				},
			},
			ShouldMatch: false,
		},
		{
			Set: apiappsv1.ReplicaSet{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "some-replicaset-postfix",
					Namespace: "non-default",
				},
			},
			ShouldMatch: false,
		},
	}

	for _, entry := range cases {
		if entry.ShouldMatch {
			assert.True(t, expr.Matches(entry.Set))
			assert.True(t, expr.Matches(&entry.Set))
		} else {
			assert.False(t, expr.Matches(entry.Set))
			assert.False(t, expr.Matches(&entry.Set))
		}
	}
}

func TestDeployment(t *testing.T) {
	type Entry struct {
		Deployment  apiappsv1.Deployment
		ShouldMatch bool
	}

	expr := deploymentExpression{
		namePattern:      "*-deployment",
		namespacePattern: "default",
	}

	cases := []Entry{
		{
			Deployment: apiappsv1.Deployment{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "some-deployment",
					Namespace: "default",
				},
			},
			ShouldMatch: true,
		},
		{
			Deployment: apiappsv1.Deployment{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "some-deployment",
					Namespace: "non-default",
				},
			},
			ShouldMatch: false,
		},
		{
			Deployment: apiappsv1.Deployment{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "some-deployment-postfix",
					Namespace: "default",
				},
			},
			ShouldMatch: false,
		},
		{
			Deployment: apiappsv1.Deployment{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "some-deployment-postfix",
					Namespace: "non-default",
				},
			},
			ShouldMatch: false,
		},
	}

	for _, entry := range cases {
		if entry.ShouldMatch {
			assert.True(t, expr.Matches(entry.Deployment))
			assert.True(t, expr.Matches(&entry.Deployment))
		} else {
			assert.False(t, expr.Matches(entry.Deployment))
			assert.False(t, expr.Matches(&entry.Deployment))
		}
	}
}

func TestService(t *testing.T) {
	type Entry struct {
		Service     apicorev1.Service
		ShouldMatch bool
	}

	expr := serviceExpression{
		namePattern:      "*-service",
		namespacePattern: "default",
	}

	cases := []Entry{
		{
			Service: apicorev1.Service{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "some-service",
					Namespace: "default",
				},
			},
			ShouldMatch: true,
		},
		{
			Service: apicorev1.Service{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "some-srvice",
					Namespace: "non-default",
				},
			},
			ShouldMatch: false,
		},
		{
			Service: apicorev1.Service{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "some-service-postfix",
					Namespace: "default",
				},
			},
			ShouldMatch: false,
		},
		{
			Service: apicorev1.Service{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "some-service-postfix",
					Namespace: "non-default",
				},
			},
			ShouldMatch: false,
		},
	}

	for _, entry := range cases {
		if entry.ShouldMatch {
			assert.True(t, expr.Matches(entry.Service), "should have matched: %s/%s", entry.Service.Namespace, entry.Service.Name)
			assert.True(t, expr.Matches(&entry.Service), "should have matched: %s/%s", entry.Service.Namespace, entry.Service.Name)
		} else {
			assert.False(t, expr.Matches(entry.Service), "should have matched: %s/%s", entry.Service.Namespace, entry.Service.Name)
			assert.False(t, expr.Matches(&entry.Service), "should have matched: %s/%s", entry.Service.Namespace, entry.Service.Name)
		}
	}
}

func TestNamespace(t *testing.T) {
	expr := namespaceExpression{
		namespacePattern: "*-ns",
	}

	assert.True(t, expr.Matches(apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Namespace: "sample-ns",
		},
	}))

	assert.False(t, expr.Matches(apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Namespace: "sample-namespace",
		},
	}))

	assert.True(t, expr.Matches(apibatchv1.Job{
		ObjectMeta: apimetav1.ObjectMeta{
			Namespace: "sample-ns",
		},
	}))

	assert.True(t, expr.Matches(apiappsv1.ReplicaSet{
		ObjectMeta: apimetav1.ObjectMeta{
			Namespace: "sample-ns",
		},
	}))

	assert.True(t, expr.Matches(apiappsv1.Deployment{
		ObjectMeta: apimetav1.ObjectMeta{
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
		ObjectMeta: apimetav1.ObjectMeta{
			Labels: map[string]string{},
		},
	}))

	assert.True(t, emptyExpr.Matches(apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
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
		ObjectMeta: apimetav1.ObjectMeta{
			Labels: map[string]string{
				simple.Key: simple.Value,
			},
		},
	}))

	assert.True(t, simpleExpr.Matches(apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Labels: map[string]string{
				simple.Key: simple.Value,
				"extra":    "extra",
			},
		},
	}))

	assert.False(t, simpleExpr.Matches(apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Labels: map[string]string{
				simple.Key: "other value",
			},
		},
	}))

	assert.False(t, simpleExpr.Matches(apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Labels: map[string]string{
				"other key": simple.Value,
			},
		},
	}))

	assert.True(t, wcKeyExpr.Matches(apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Labels: map[string]string{
				wcKey.Key: wcKey.Value,
			},
		},
	}))

	assert.False(t, wcKeyExpr.Matches(apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Labels: map[string]string{
				wcKey.Key: "other value",
			},
		},
	}))

	assert.False(t, wcKeyExpr.Matches(apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Labels: map[string]string{
				"other key": wcKey.Value,
			},
		},
	}))

	assert.True(t, wcValueExpr.Matches(apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Labels: map[string]string{
				wcValue.Key: wcValue.Value,
			},
		},
	}))

	assert.False(t, wcValueExpr.Matches(apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Labels: map[string]string{
				wcValue.Key: "other value",
			},
		},
	}))

	assert.False(t, wcValueExpr.Matches(apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
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
		ObjectMeta: apimetav1.ObjectMeta{
			Labels: map[string]string{
				simple.Key:  simple.Value,
				wcKey.Key:   wcKey.Value,
				wcValue.Key: wcValue.Value,
			},
		},
	}))

	// 1 0 1 (failed value)
	assert.False(t, union.Matches(apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Labels: map[string]string{
				simple.Key:  simple.Value,
				wcKey.Key:   "other value",
				wcValue.Key: wcValue.Value,
			},
		},
	}))

	// 1 0 1 (failed key)
	assert.False(t, union.Matches(apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Labels: map[string]string{
				simple.Key:  simple.Value,
				"other key": wcKey.Key,
				wcValue.Key: wcValue.Value,
			},
		},
	}))

	// missing the simple
	assert.False(t, union.Matches(apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Labels: map[string]string{
				wcKey.Key:   wcKey.Key,
				wcValue.Key: wcValue.Value,
			},
		},
	}))
}
