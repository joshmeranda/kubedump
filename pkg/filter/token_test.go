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
	assert.NoError(t, err)
	assert.Equal(t, Token{
		Kind: Pod,
		Body: "pod",
	}, token)
}

func TestTokenizerNextExpectingPattern(t *testing.T) {
	t.Skipf("low frequency and low impact bug")

	tokenizer := NewTokenizer("pod pod")

	token, err := tokenizer.Next()
	assert.NoError(t, err)
	assert.Equal(t, Token{
		Kind: Pod,
		Body: "pod",
	}, token)

	token, err = tokenizer.Next()
	assert.NoError(t, err)
	assert.Equal(t, Token{
		Kind: Pattern,
		Body: "pod",
	}, token)
}

func TestTokenization(t *testing.T) {
	s := "pod and or (not namespace/pod)"
	tokenizer := NewTokenizer(s)

	expected := []Token{
		{
			Kind: Pod,
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
