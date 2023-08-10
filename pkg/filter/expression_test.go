package filter

import (
	"testing"

	kubedump "github.com/joshmeranda/kubedump/pkg"
	"github.com/stretchr/testify/assert"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Entry struct {
	Resource    kubedump.HandledResource
	ShouldMatch bool
}

func TestNot(t *testing.T) {
	assert.True(t, notExpression{
		inner: falsyExpression{},
	}.Matches(kubedump.HandledResource{}))

	assert.False(t, notExpression{
		inner: truthyExpression{},
	}.Matches(kubedump.HandledResource{}))
}

func TestAnd(t *testing.T) {
	assert.True(t, andExpression{
		left:  truthyExpression{},
		right: truthyExpression{},
	}.Matches(kubedump.HandledResource{}))

	assert.False(t, andExpression{
		left:  falsyExpression{},
		right: truthyExpression{},
	}.Matches(kubedump.HandledResource{}))

	assert.False(t, andExpression{
		left:  truthyExpression{},
		right: falsyExpression{},
	}.Matches(kubedump.HandledResource{}))

	assert.False(t, andExpression{
		left:  falsyExpression{},
		right: falsyExpression{},
	}.Matches(kubedump.HandledResource{}))
}

func TestOr(t *testing.T) {
	assert.True(t, orExpression{
		left:  truthyExpression{},
		right: truthyExpression{},
	}.Matches(kubedump.HandledResource{}))

	assert.True(t, orExpression{
		left:  falsyExpression{},
		right: truthyExpression{},
	}.Matches(kubedump.HandledResource{}))

	assert.True(t, orExpression{
		left:  truthyExpression{},
		right: falsyExpression{},
	}.Matches(kubedump.HandledResource{}))

	assert.False(t, orExpression{
		left:  falsyExpression{},
		right: falsyExpression{},
	}.Matches(kubedump.HandledResource{}))
}

func TestResource(t *testing.T) {
	expr := resourceExpression{
		kind:             "Pod",
		namePattern:      "*-pod",
		namespacePattern: "default",
	}

	cases := []Entry{
		{
			Resource: kubedump.HandledResource{
				Object: &apimetav1.ObjectMeta{
					Name:      "some-pod",
					Namespace: "default",
				},
				TypeMeta: apimetav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				Resource: apicorev1.Pod{},
			},
			ShouldMatch: true,
		},
		{
			Resource: kubedump.HandledResource{
				Object: &apimetav1.ObjectMeta{
					Name:      "some-pod",
					Namespace: "non-default",
				},
				TypeMeta: apimetav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				Resource: apicorev1.Pod{},
			},
			ShouldMatch: false,
		},
		{

			Resource: kubedump.HandledResource{
				Object: &apimetav1.ObjectMeta{
					Name:      "some-pod-postfix",
					Namespace: "default",
				},
				TypeMeta: apimetav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				Resource: apicorev1.Pod{},
			},
			ShouldMatch: false,
		},
		{
			Resource: kubedump.HandledResource{
				Object: &apimetav1.ObjectMeta{
					Name:      "some-pod-postfix",
					Namespace: "non-default",
				},
				TypeMeta: apimetav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				Resource: apicorev1.Pod{},
			},
			ShouldMatch: false,
		},

		{
			Resource: kubedump.HandledResource{
				Object: &apimetav1.ObjectMeta{
					Name:      "some-pod",
					Namespace: "default",
				},
				TypeMeta: apimetav1.TypeMeta{
					Kind:       "Job",
					APIVersion: "v1",
				},
				Resource: apibatchv1.Job{},
			},
			ShouldMatch: false,
		},
	}

	for _, entry := range cases {
		if entry.ShouldMatch {
			assert.True(t, expr.Matches(entry.Resource), "should be true: "+entry.Resource.String())
		} else {
			assert.False(t, expr.Matches(entry.Resource), "should be false: "+entry.Resource.String())
		}
	}
}
