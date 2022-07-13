package filter

import (
	"github.com/stretchr/testify/assert"
	kubedump "kubedump/pkg"
	"testing"
)

func TestPrefix(t *testing.T) {
	// not pod */some-pod or (pod */another-pod or pod default/*)
	// default/* pod */another-pod pod or */some-pod pod not or
	// or not pod */some-pod or pod */another-pod pod default/*
	tokens := []token{
		{
			Kind: Operator,
			Body: "not",
		},
		{
			Kind: Resource,
			Body: string(kubedump.ResourcePod),
		},
		{
			Kind: Pattern,
			Body: "*/some-pod",
		},
		{
			Kind: Operator,
			Body: "or",
		},
		{
			Kind: OpenParenthesis,
			Body: "(",
		},
		{
			Kind: Resource,
			Body: string(kubedump.ResourcePod),
		},
		{
			Kind: Pattern,
			Body: "*/another-pod",
		},
		{
			Kind: Operator,
			Body: "or",
		},
		{
			Kind: Resource,
			Body: string(kubedump.ResourcePod),
		},
		{
			Kind: Pattern,
			Body: "default/*",
		},
		{
			Kind: CloseParenthesis,
			Body: ")",
		},
		{
			Kind: EOE,
			Body: "EOE",
		},
	}

	expected := []token{
		{
			Kind: Operator,
			Body: "or",
		},
		{
			Kind: Operator,
			Body: "not",
		},
		{
			Kind: Resource,
			Body: string(kubedump.ResourcePod),
		},
		{
			Kind: Pattern,
			Body: "*/some-pod",
		},
		{
			Kind: Operator,
			Body: "or",
		},
		{
			Kind: Resource,
			Body: string(kubedump.ResourcePod),
		},
		{
			Kind: Pattern,
			Body: "*/another-pod",
		},
		{
			Kind: Resource,
			Body: string(kubedump.ResourcePod),
		},
		{
			Kind: Pattern,
			Body: "default/*",
		},
		{
			Kind: EOE,
			Body: "EOE",
		},
	}
	actual := prefixTokens(tokens)

	assert.Equal(t, expected, actual)
}

func TestSplitPattern(t *testing.T) {
	namespace, name := splitPattern("namespace/name/name")
	assert.Equal(t, "namespace", namespace)
	assert.Equal(t, "name/name", name)

	namespace, name = splitPattern("namespace/name")
	assert.Equal(t, "namespace", namespace)
	assert.Equal(t, "name", name)

	namespace, name = splitPattern("name")
	assert.Equal(t, "default", namespace)
	assert.Equal(t, "name", name)

	namespace, name = splitPattern("")
	assert.Zero(t, namespace)
	assert.Zero(t, name)
}

func TestParseEmpty(t *testing.T) {
	expr, err := Parse("")

	assert.NoError(t, err)
	assert.Equal(t, truthyExpression{}, expr)
}

func TestParseSimple(t *testing.T) {
	expr, err := Parse("pod default/*")

	assert.NoError(t, err)
	assert.Equal(t, podExpression{
		NamePattern:      "*",
		NamespacePattern: "default",
	}, expr)
}

func TestComplex(t *testing.T) {
	expr, err := Parse("not pod */pod-name and (pod another-pod or job namespace/some-job)")

	assert.NoError(t, err)
	assert.Equal(t, andExpression{
		Left: notExpression{
			Inner: podExpression{
				NamePattern:      "pod-name",
				NamespacePattern: "*",
			},
		},
		Right: orExpression{
			Left: podExpression{
				NamePattern:      "another-pod",
				NamespacePattern: "default",
			},
			Right: jobExpression{
				NamePattern:      "some-job",
				NamespacePattern: "namespace",
			},
		},
	}, expr)
}

func TestParseBadExpression(t *testing.T) {
	expr, err := Parse("and and")

	assert.Error(t, err)
	assert.Nil(t, expr)
}
