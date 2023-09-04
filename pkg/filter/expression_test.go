package filter

import (
	"testing"

	kubedump "github.com/joshmeranda/kubedump/pkg"
	"github.com/stretchr/testify/assert"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Entry struct {
	Resource    kubedump.Resource
	ShouldMatch bool
}

func TestNot(t *testing.T) {
	assert.True(t, notExpression{
		inner: falsyExpression{},
	}.Matches(kubedump.NewResourceBuilder().Build()))

	assert.False(t, notExpression{
		inner: truthyExpression{},
	}.Matches(kubedump.NewResourceBuilder().Build()))
}

func TestAnd(t *testing.T) {
	assert.True(t, andExpression{
		left:  truthyExpression{},
		right: truthyExpression{},
	}.Matches(kubedump.NewResourceBuilder().Build()))

	assert.False(t, andExpression{
		left:  falsyExpression{},
		right: truthyExpression{},
	}.Matches(kubedump.NewResourceBuilder().Build()))

	assert.False(t, andExpression{
		left:  truthyExpression{},
		right: falsyExpression{},
	}.Matches(kubedump.NewResourceBuilder().Build()))

	assert.False(t, andExpression{
		left:  falsyExpression{},
		right: falsyExpression{},
	}.Matches(kubedump.NewResourceBuilder().Build()))
}

func TestOr(t *testing.T) {
	assert.True(t, orExpression{
		left:  truthyExpression{},
		right: truthyExpression{},
	}.Matches(kubedump.NewResourceBuilder().Build()))

	assert.True(t, orExpression{
		left:  falsyExpression{},
		right: truthyExpression{},
	}.Matches(kubedump.NewResourceBuilder().Build()))

	assert.True(t, orExpression{
		left:  truthyExpression{},
		right: falsyExpression{},
	}.Matches(kubedump.NewResourceBuilder().Build()))

	assert.False(t, orExpression{
		left:  falsyExpression{},
		right: falsyExpression{},
	}.Matches(kubedump.NewResourceBuilder().Build()))
}

func TestResource(t *testing.T) {
	expr := resourceExpression{
		kind:             "Pod",
		namePattern:      "*-pod",
		namespacePattern: "default",
	}

	cases := []Entry{
		{
			Resource: kubedump.NewResourceBuilder().
				FromObject(apimetav1.ObjectMeta{
					Name:      "some-pod",
					Namespace: "default",
				}).
				FromType(apimetav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				}).
				Build(),
			ShouldMatch: true,
		},
		{
			Resource: kubedump.NewResourceBuilder().
				FromObject(apimetav1.ObjectMeta{
					Name:      "some-pod",
					Namespace: "non-default",
				}).
				FromType(apimetav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				}).
				Build(),
			ShouldMatch: false,
		},
		{

			Resource: kubedump.NewResourceBuilder().
				FromObject(apimetav1.ObjectMeta{
					Name:      "some-pod-postfix",
					Namespace: "default",
				}).
				FromType(apimetav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				}).
				Build(),
			ShouldMatch: false,
		},
		{
			Resource: kubedump.NewResourceBuilder().
				FromObject(apimetav1.ObjectMeta{
					Name:      "some-pod-postfix",
					Namespace: "non-default",
				}).
				FromType(apimetav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				}).
				Build(),
			ShouldMatch: false,
		},
		{
			Resource: kubedump.NewResourceBuilder().
				FromObject(apimetav1.ObjectMeta{
					Name:      "some-pod",
					Namespace: "default",
				}).
				FromType(apimetav1.TypeMeta{
					Kind:       "Job",
					APIVersion: "v1",
				}).
				Build(),
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
