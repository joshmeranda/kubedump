package filter

import (
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseEmpty(t *testing.T) {
	expr, err := Parse("")

	assert.NoError(t, err)
	assert.Equal(t, truthyExpression{}, expr)
}

func TestComplex(t *testing.T) {
	expr, err := Parse("not pod */pod-name and (pod another-pod or job namespace/some-job or replicaset *)")

	assert.NoError(t, err)
	assert.Equal(t, andExpression{
		Left: notExpression{
			Inner: podExpression{
				NamePattern:      "pod-name",
				NamespacePattern: "*",
			},
		},
		Right: orExpression{
			Left: orExpression{
				Left: podExpression{
					NamePattern:      "another-pod",
					NamespacePattern: "default",
				},
				Right: jobExpression{
					NamePattern:      "some-job",
					NamespacePattern: "namespace",
				},
			},
			Right: replicasetExpression{
				NamePattern:      "*",
				NamespacePattern: "default",
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
			ExpectedExpr: podExpression{
				NamespacePattern: "*",
				NamePattern:      "*",
			},
		},
		{
			Expr: "pod *",
			ExpectedExpr: podExpression{
				NamespacePattern: "default",
				NamePattern:      "*",
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
		Left: podExpression{
			NamePattern:      "*",
			NamespacePattern: "default",
		},
		Right: podExpression{
			NamePattern:      "*",
			NamespacePattern: "default",
		},
	}, expr)

	expr, err = Parse("pod * and")
	assert.Error(t, err)
	assert.Nil(t, expr)

	expr, err = Parse("pod * or pod *")
	assert.NoError(t, err)
	assert.Equal(t, orExpression{
		Left: podExpression{
			NamePattern:      "*",
			NamespacePattern: "default",
		},
		Right: podExpression{
			NamePattern:      "*",
			NamespacePattern: "default",
		},
	}, expr)

	expr, err = Parse("pod * or")
	assert.Error(t, err)
	assert.Nil(t, expr)
}

func TestParsedChainedOperatorExpression(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)

	expr, err := Parse("pod a and pod b and pod c")
	assert.NoError(t, err)

	expected := andExpression{
		Left: andExpression{
			Left: podExpression{
				NamePattern:      "a",
				NamespacePattern: "default",
			},
			Right: podExpression{
				NamePattern:      "b",
				NamespacePattern: "default",
			},
		},
		Right: podExpression{
			NamePattern:      "c",
			NamespacePattern: "default",
		},
	}

	assert.Equal(t, expected, expr)
}

func TestParseNotExpression(t *testing.T) {
	expr, err := Parse("not pod *")
	assert.NoError(t, err)
	assert.Equal(t, notExpression{
		Inner: podExpression{
			NamePattern:      "*",
			NamespacePattern: "default",
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
		NamespacePattern: "default",
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
