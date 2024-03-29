package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseEmpty(t *testing.T) {
	expr, err := Parse("")

	assert.NoError(t, err)
	assert.Equal(t, truthyExpression{}, expr)
}

func TestComplex(t *testing.T) {
	expr, err := Parse("not pod a and (pod b or job c or replicaset d)")

	assert.NoError(t, err)
	assert.Equal(t, andExpression{
		left: notExpression{
			inner: resourceExpression{
				kind:             "pod",
				namePattern:      "a",
				namespacePattern: "default",
			},
		},
		right: orExpression{
			left: resourceExpression{
				kind:             "pod",
				namePattern:      "b",
				namespacePattern: "default",
			},
			right: orExpression{
				left: resourceExpression{
					kind:             "job",
					namePattern:      "c",
					namespacePattern: "default",
				},
				right: resourceExpression{
					kind:             "replicaset",
					namePattern:      "d",
					namespacePattern: "default",
				},
			},
		},
	}, expr)
}

func TestParseResourceExpression(t *testing.T) {
	type Case struct {
		Expr         string
		ExpectedExpr Expression
		ExpectsError bool
	}

	// since all resource expressions are parsed the same way, we only need to make sure it works for one of them
	cases := []Case{
		{
			Expr: "pod */*",
			ExpectedExpr: resourceExpression{
				kind:             "pod",
				namespacePattern: "*",
				namePattern:      "*",
			},
		},
		{
			Expr: "pod *",
			ExpectedExpr: resourceExpression{
				kind:             "pod",
				namespacePattern: "default",
				namePattern:      "*",
			},
		},
		{
			Expr:         "pod",
			ExpectsError: true,
		},
		{
			Expr:         "pod * *",
			ExpectsError: true,
		},
		{
			Expr:         "pod bad-name-",
			ExpectsError: true,
		},
	}

	for _, c := range cases {
		expr, err := Parse(c.Expr)

		if c.ExpectsError {
			assert.Nil(t, expr, "expected nil for expression '%s'", c.Expr)
			assert.Error(t, err, "expected an error for expression '%s'", c.Expr)
		} else {
			assert.Equal(t, c.ExpectedExpr, expr)
			assert.NoError(t, err)
		}
	}
}

func TestParseOperatorExpression(t *testing.T) {
	expr, err := Parse("pod * and pod *")
	assert.NoError(t, err)
	assert.Equal(t, andExpression{
		left: resourceExpression{
			kind:             "pod",
			namePattern:      "*",
			namespacePattern: "default",
		},
		right: resourceExpression{
			kind:             "pod",
			namePattern:      "*",
			namespacePattern: "default",
		},
	}, expr)

	expr, err = Parse("pod * and")
	assert.Error(t, err)
	assert.Nil(t, expr)

	expr, err = Parse("pod * or pod *")
	assert.NoError(t, err)
	assert.Equal(t, orExpression{
		left: resourceExpression{
			kind:             "pod",
			namePattern:      "*",
			namespacePattern: "default",
		},
		right: resourceExpression{
			kind:             "pod",
			namePattern:      "*",
			namespacePattern: "default",
		},
	}, expr)

	expr, err = Parse("pod * or")
	assert.Error(t, err)
	assert.Nil(t, expr)
}

func TestParsedChainedOperatorExpression(t *testing.T) {
	expr, err := Parse("pod a and pod b and pod c")
	assert.NoError(t, err)

	expected := andExpression{
		left: resourceExpression{
			kind:             "pod",
			namePattern:      "a",
			namespacePattern: "default",
		},
		right: andExpression{
			left: resourceExpression{
				kind:             "pod",
				namePattern:      "b",
				namespacePattern: "default",
			},
			right: resourceExpression{
				kind:             "pod",
				namePattern:      "c",
				namespacePattern: "default",
			},
		},
	}

	assert.Equal(t, expected, expr)
}

func TestParseNotExpression(t *testing.T) {
	expr, err := Parse("not pod *")
	assert.NoError(t, err)
	assert.Equal(t, notExpression{
		inner: resourceExpression{
			kind:             "pod",
			namePattern:      "*",
			namespacePattern: "default",
		},
	}, expr)

	expr, err = Parse("not")
	assert.Error(t, err)
	assert.Nil(t, expr)
}

func TestParseNamespaceExpression(t *testing.T) {
	expr, err := Parse("namespace default")
	assert.NoError(t, err)
	assert.Equal(t, namespaceExpression{
		namespacePattern: "default",
	}, expr)

	expr, err = Parse("namespace")
	assert.Error(t, err)
	assert.Nil(t, expr)

	expr, err = Parse("namespace bad-namespace-name-")
	assert.Error(t, err)
	assert.Nil(t, expr)
}

func TestParseLabelExpression(t *testing.T) {
	expr, err := Parse("label app=kubedump *-wc-key=*-wc-pattern empty=")

	assert.Error(t, err)
	assert.Nil(t, expr)

	expr, err = Parse("label")

	assert.Error(t, err)
	assert.Nil(t, expr)

	expr, err = Parse("label resource=pod")
	assert.NoError(t, err)
	assert.Equal(t, labelExpression{
		labels: map[string]string{
			"resource": "pod",
		},
	}, expr)

	expr, err = Parse("label =bad")
	assert.Error(t, err)
	assert.Nil(t, expr)
}
