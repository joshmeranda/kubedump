package filter

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLexNextWithExcessWhitespace(t *testing.T) {
	lval := &yySymType{}
	lexer := NewLexer("  pod    default/*")

	assert.Equal(t, RESOURCE, lexer.Lex(lval))
	assert.Equal(t, "pod", lval.s)

	assert.Equal(t, PATTERN, lexer.Lex(lval))
	assert.Equal(t, lval.s, "default/*")

	assert.Equal(t, lexer.Lex(lval), EOF)
}

func TestLex(t *testing.T) {
	lval := &yySymType{}
	lexer := NewLexer("pod job deployment replicaset service configmap and or (not namespace/pod) namespace label a=b")

	assert.Equal(t, RESOURCE, lexer.Lex(lval))
	assert.Equal(t, "pod", lval.s)

	assert.Equal(t, RESOURCE, lexer.Lex(lval))
	assert.Equal(t, "job", lval.s)

	assert.Equal(t, RESOURCE, lexer.Lex(lval))
	assert.Equal(t, "deployment", lval.s)

	assert.Equal(t, RESOURCE, lexer.Lex(lval))
	assert.Equal(t, "replicaset", lval.s)

	assert.Equal(t, RESOURCE, lexer.Lex(lval))
	assert.Equal(t, "service", lval.s)

	assert.Equal(t, RESOURCE, lexer.Lex(lval))
	assert.Equal(t, "configmap", lval.s)

	assert.Equal(t, AND, lexer.Lex(lval))

	assert.Equal(t, OR, lexer.Lex(lval))

	assert.Equal(t, int('('), lexer.Lex(lval))

	assert.Equal(t, NOT, lexer.Lex(lval))

	assert.Equal(t, PATTERN, lexer.Lex(lval))
	assert.Equal(t, "namespace/pod", lval.s)

	assert.Equal(t, int(')'), lexer.Lex(lval))

	assert.Equal(t, NAMESPACE, lexer.Lex(lval))
	assert.Equal(t, "namespace", lval.s)

	assert.Equal(t, LABEL, lexer.Lex(lval))

	assert.Equal(t, PATTERN, lexer.Lex(lval))
	assert.Equal(t, "a=b", lval.s)
}
