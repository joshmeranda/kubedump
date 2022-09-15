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
		Kind:   Resource,
		Body:   "pod",
		Column: 2,
	}, nextToken)

	nextToken, err = tokenizer.Next()
	assert.NoError(t, err)
	assert.Equal(t, token{
		Kind:   Pattern,
		Body:   "default/*",
		Column: 9,
	}, nextToken)
}

func TestTokenize(t *testing.T) {
	s := "pod job and or (not namespace/pod) namespace label a=b"
	tokenizer := newTokenizer(s)

	expected := []token{
		{
			Kind:   Resource,
			Body:   "pod",
			Column: 0,
		},
		{
			Kind:   Resource,
			Body:   "job",
			Column: 4,
		},
		{
			Kind:   Operator,
			Body:   "and",
			Column: 8,
		},
		{
			Kind:   Operator,
			Body:   "or",
			Column: 12,
		},
		{
			Kind:   OpenParenthesis,
			Body:   "(",
			Column: 15,
		},
		{
			Kind:   Operator,
			Body:   "not",
			Column: 16,
		},
		{
			Kind:   Pattern,
			Body:   "namespace/pod",
			Column: 20,
		},
		{
			Kind:   CloseParenthesis,
			Body:   ")",
			Column: 33,
		},
		{
			Kind:   Namespace,
			Body:   "namespace",
			Column: 35,
		},
		{
			Kind:   Label,
			Body:   "label",
			Column: 45,
		},
		{
			Kind:   Pattern,
			Body:   "a=b",
			Column: 51,
		},
		{
			Kind:   EOE,
			Column: 54,
		},
	}
	actual, err := tokenizer.Tokenize()

	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}
