package filter

import (
	"fmt"
	"strings"
	"unicode"
)

type tokenKind int

const (
	Pattern tokenKind = iota
	Operator

	Namespace

	Resource

	OpenParenthesis
	CloseParenthesis

	Label

	// EOE is the End Of Expression
	EOE
)

type token struct {
	Kind tokenKind
	Body string
}

type tokenizer struct {
	s    string
	head int
}

func newTokenizer(s string) tokenizer {
	return tokenizer{
		s: s,
	}
}

func (tokenizer *tokenizer) nextNonSpace() int {
	nextHead := tokenizer.head

	for ; nextHead < len(tokenizer.s); nextHead++ {
		if !unicode.IsSpace(rune(tokenizer.s[nextHead])) {
			return nextHead
		}
	}

	return nextHead
}

func (tokenizer *tokenizer) Next() (token, error) {
	tokenizer.head = tokenizer.nextNonSpace()

	if tokenizer.head == len(tokenizer.s) {
		return token{
			Kind: EOE,
		}, nil
	} else if tokenizer.s[tokenizer.head] == '(' {
		tokenizer.head++

		return token{
			Kind: OpenParenthesis,
			Body: "(",
		}, nil
	} else if tokenizer.s[tokenizer.head] == ')' {
		tokenizer.head++

		return token{
			Kind: CloseParenthesis,
			Body: ")",
		}, nil
	}

	nextHead := strings.IndexFunc(tokenizer.s[tokenizer.head:], func(r rune) bool {
		return unicode.IsSpace(r) || r == '(' || r == ')'
	})

	if nextHead == -1 {
		nextHead = len(tokenizer.s) - 1
	} else {
		nextHead += tokenizer.head - 1
	}

	body := tokenizer.s[tokenizer.head : nextHead+1]

	tokenizer.head = nextHead + 1

	var t token
	switch body {
	case "pod", "job", "deployment":
		t = token{
			Kind: Resource,
			Body: body,
		}
	case "not", "and", "or":
		t = token{
			Kind: Operator,
			Body: body,
		}
	case "namespace":
		t = token{
			Kind: Namespace,
			Body: body,
		}
	case "label":
		t = token{
			Kind: Label,
			Body: "label",
		}
	default:
		t = token{
			Kind: Pattern,
			Body: body,
		}
	}

	return t, nil
}

func (tokenizer *tokenizer) Tokenize() ([]token, error) {
	var tokens []token

	for {
		token, err := tokenizer.Next()

		if err != nil {
			return nil, fmt.Errorf("error during tokenization: %w", err)
		}

		tokens = append(tokens, token)

		if token.Kind == EOE {
			break
		}
	}

	return tokens, nil
}
