package filter

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTokenizeNextWithExcessWhitespace(t *testing.T) {
	tokenizer := newTokenizer("  pod    default/*")

	nextToken, err := tokenizer.Next()
	assert.NoError(t, err)
	assert.Equal(t, token{
		Kind: Resource,
		Body: "pod",
	}, nextToken)

	nextToken, err = tokenizer.Next()
	assert.NoError(t, err)
	assert.Equal(t, token{
		Kind: Pattern,
		Body: "default/*",
	}, nextToken)
}

func TestTokenize(t *testing.T) {
	s := "pod job deployment replicaset service and or (not namespace/pod) namespace label a=b"
	tokenizer := newTokenizer(s)

	expected := []token{
		{
			Kind: Resource,
			Body: "pod",
		},
		{
			Kind: Resource,
			Body: "job",
		},
		{
			Kind: Resource,
			Body: "deployment",
		},
		{
			Kind: Resource,
			Body: "replicaset",
		},
		{
			Kind: Resource,
			Body: "service",
		},
		{
			Kind: Operator,
			Body: "and",
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
			Kind: Operator,
			Body: "not",
		},
		{
			Kind: Pattern,
			Body: "namespace/pod",
		},
		{
			Kind: CloseParenthesis,
			Body: ")",
		},
		{
			Kind: Namespace,
			Body: "namespace",
		},
		{
			Kind: Label,
			Body: "label",
		},
		{
			Kind: Pattern,
			Body: "a=b",
		},
		{
			Kind: EOE,
		},
	}
	actual, err := tokenizer.Tokenize()

	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}
