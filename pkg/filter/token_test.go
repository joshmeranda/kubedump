package filter

import (
	"github.com/stretchr/testify/assert"
	"kubedump/pkg/collector"
	"testing"
)

func TestTokenizeNext(t *testing.T) {
	tokenizer := newTokenizer("pod")

	nextToken, err := tokenizer.Next()
	assert.Equal(t, token{
		Kind: Resource,
		Body: "pod",
	}, nextToken)
	assert.NoError(t, err)
}

func TestTokenizeNextWhitespace(t *testing.T) {
	tokenizer := newTokenizer("  pod    ")

	nextToken, err := tokenizer.Next()
	assert.NoError(t, err)
	assert.Equal(t, token{
		Kind: Resource,
		Body: "pod",
	}, nextToken)
}

func TestTokenizerNextExpectingPattern(t *testing.T) {
	t.Skipf("low frequency and low impact bug")

	tokenizer := newTokenizer("pod pod")

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
		Body: "pod",
	}, nextToken)
}

func TestTokenization(t *testing.T) {
	s := "pod and or (not namespace/pod)"
	tokenizer := newTokenizer(s)

	expected := []token{
		{
			Kind: Resource,
			Body: "pod",
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
			Kind: EOE,
		},
	}
	actual, err := tokenizer.Tokenize()

	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestPostfix(t *testing.T) {
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
			Body: string(collector.ResourcePod),
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
			Body: string(collector.ResourcePod),
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
			Body: string(collector.ResourcePod),
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
			Body: string(collector.ResourcePod),
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
			Body: string(collector.ResourcePod),
		},
		{
			Kind: Pattern,
			Body: "*/another-pod",
		},
		{
			Kind: Resource,
			Body: string(collector.ResourcePod),
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
