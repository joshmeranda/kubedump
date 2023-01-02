package controller

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEmptyLabelSet(t *testing.T) {
	matcher, err := MatcherFromLabels(map[string]string{})
	assert.Error(t, err)
	assert.Nil(t, matcher)
}

func TestLabelSet(t *testing.T) {
	matcher, err := MatcherFromLabels(map[string]string{"a": "b"})
	assert.NoError(t, err)

	assert.False(t, matcher.Matches(map[string]string{}))
	assert.False(t, matcher.Matches(map[string]string{"some-key": "some-value"}))
	assert.False(t, matcher.Matches(map[string]string{"a": "c"}))
	assert.False(t, matcher.Matches(map[string]string{"c": "b"}))

	assert.True(t, matcher.Matches(map[string]string{"a": "b"}))
}
