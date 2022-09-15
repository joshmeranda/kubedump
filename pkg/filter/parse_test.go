package filter

import (
	"github.com/stretchr/testify/assert"
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
			Body: "Pod",
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
			Body: "Pod",
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
			Body: "Pod",
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
			Body: "Pod",
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
			Body: "Pod",
		},
		{
			Kind: Pattern,
			Body: "*/another-pod",
		},
		{
			Kind: Resource,
			Body: "Pod",
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

	expr, err = Parse("namespace default")

	assert.NoError(t, err)
	assert.Equal(t, namespaceExpression{
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

	expr, err = Parse("pod")
	assert.Error(t, err)
	assert.Nil(t, expr)

	expr, err = Parse("namespace")
	assert.Error(t, err)
	assert.Nil(t, expr)
}

func TestParseLabelExpression(t *testing.T) {
	expr, err := Parse("label app=kubedump *-wc-key=*-wc-pattern empty=")

	assert.NoError(t, err)
	assert.Equal(t, labelExpression{
		labelPatterns: map[string]string{
			"app":      "kubedump",
			"*-wc-key": "*-wc-pattern",
			"empty":    "",
		},
	}, expr)

	expr, err = Parse("label")

	assert.NoError(t, err)
	assert.Equal(t, labelExpression{
		labelPatterns: map[string]string{},
	}, expr)

	expr, err = Parse("label resource=pod")
	assert.NoError(t, err)
	assert.Equal(t, labelExpression{
		labelPatterns: map[string]string{
			"resource": "pod",
		},
	}, expr)

	expr, err = Parse("label =bad")
	assert.Error(t, err)
	assert.Nil(t, expr)
}
