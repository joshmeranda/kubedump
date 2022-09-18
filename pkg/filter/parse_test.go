package filter

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseEmpty(t *testing.T) {
	expr, err := Parse("")

	assert.NoError(t, err)
	assert.Equal(t, truthyExpression{}, expr)
}

func TestParseResourceExpression(t *testing.T) {
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
