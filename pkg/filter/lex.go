package filter

import (
	"fmt"
	"strings"
	"unicode"
)

type Lexer struct {
	s    string
	head int

	err    error
	result Expression
}

func NewLexer(s string) Lexer {
	return Lexer{
		s: s,
	}
}

func (lexer *Lexer) nextNonSpace() {
	for ; lexer.head < len(lexer.s); lexer.head++ {
		if !unicode.IsSpace(rune(lexer.s[lexer.head])) {
			return
		}
	}
}

func (lexer *Lexer) Lex(lval *yySymType) int {
	lexer.nextNonSpace()

	if lexer.head == len(lexer.s) {
		return EOF
	}

	switch lexer.s[lexer.head] {
	case '(', ')':
		lexer.head++
		return int(lexer.s[lexer.head-1])
	}

	nextHead := strings.IndexFunc(lexer.s[lexer.head:], func(r rune) bool {
		return unicode.IsSpace(r) || r == '(' || r == ')'
	})

	if nextHead == -1 {
		nextHead = len(lexer.s) - 1
	} else {
		nextHead += lexer.head - 1
	}

	body := lexer.s[lexer.head : nextHead+1]
	lexer.head = nextHead + 1

	lval.s = body
	switch body {
	case "pod", "job", "deployment", "replicaset", "service", "configmap", "secret":
		return RESOURCE
	case "namespace":
		return NAMESPACE
	case "label":
		return LABEL
	case "not":
		return NOT
	case "or":
		return OR
	case "and":
		return AND
	default:
		return PATTERN
	}
}

func (lexer *Lexer) Error(s string) {
	lexer.err = fmt.Errorf(s)
}
