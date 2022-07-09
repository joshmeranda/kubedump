package filter

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTokenizeNext(t *testing.T) {
	tokenizer := NewTokenizer("pod")

	token, err := tokenizer.Next()
	assert.Equal(t, Token{
		Kind: Pod,
		Body: "pod",
	}, token)
	assert.NoError(t, err)
}

func TestTokenizeNextWhitespace(t *testing.T) {
	tokenizer := NewTokenizer("  pod    ")

	token, err := tokenizer.Next()
	assert.Equal(t, Token{
		Kind: Pod,
		Body: "pod",
	}, token)
	assert.NoError(t, err)
}

func TestTokenization(t *testing.T) {
	s := "pod namespace and or (not namespace/pod)"
	tokenizer := NewTokenizer(s)

	expected := []Token{
		{
			Kind: Pod,
			Body: "pod",
		},
		{
			Kind: Namespace,
			Body: "namespace",
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
			Kind: Not,
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
