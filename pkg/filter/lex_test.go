package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLexNextWithExcessWhitespace(t *testing.T) {
	lval := &yySymType{}
	lexer := NewLexer("  pod    default/*")

	assert.Equal(t, IDENTIFIER, lexer.Lex(lval))
	assert.Equal(t, "pod", lval.s)

	assert.Equal(t, IDENTIFIER, lexer.Lex(lval))
	assert.Equal(t, lval.s, "default/*")

	assert.Equal(t, lexer.Lex(lval), EOF)
}

func TestLex(t *testing.T) {
	lval := &yySymType{}
	// lexer := NewLexer("pod job deployment replicaset service configmap secret and or (not namespace/pod) namespace label a=b")
	lexer := NewLexer("and or (not pod namespace/name) namespace label a=b")

	assert.Equal(t, AND, lexer.Lex(lval))

	assert.Equal(t, OR, lexer.Lex(lval))

	assert.Equal(t, int('('), lexer.Lex(lval))

	assert.Equal(t, NOT, lexer.Lex(lval))

	assert.Equal(t, IDENTIFIER, lexer.Lex(lval))
	assert.Equal(t, "pod", lval.s)

	assert.Equal(t, IDENTIFIER, lexer.Lex(lval))
	assert.Equal(t, "namespace/name", lval.s)

	assert.Equal(t, int(')'), lexer.Lex(lval))

	assert.Equal(t, NAMESPACE, lexer.Lex(lval))
	assert.Equal(t, "namespace", lval.s)

	assert.Equal(t, LABEL, lexer.Lex(lval))

	assert.Equal(t, IDENTIFIER, lexer.Lex(lval))
	assert.Equal(t, "a=b", lval.s)
}
