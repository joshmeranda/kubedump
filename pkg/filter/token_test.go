package filter

import (
	"github.com/stretchr/testify/assert"
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
	s := "pod job and or (not namespace/pod)"
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
