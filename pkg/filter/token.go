package filter

import (
	"fmt"
	"strings"
	"unicode"
)

type TokenKind int

const (
	Pattern TokenKind = iota
	Operator
	Not

	Pod
	Namespace

	OpenParenthesis
	CloseParenthesis

	// EOE is the End Of Expression
	EOE
)

type Token struct {
	Kind TokenKind
	Body string
}

type Tokenizer struct {
	s    string
	head int
}

func NewTokenizer(s string) Tokenizer {
	return Tokenizer{
		s: s,
	}
}

func (tokenizer *Tokenizer) nextNonSpace() int {
	nextHead := tokenizer.head

	for ; nextHead < len(tokenizer.s); nextHead++ {
		if !unicode.IsSpace(rune(tokenizer.s[nextHead])) {
			return nextHead
		}
	}

	return nextHead
}

func (tokenizer *Tokenizer) Next() (Token, error) {
	tokenizer.head = tokenizer.nextNonSpace()

	if tokenizer.head == len(tokenizer.s) {
		return Token{
			Kind: EOE,
		}, nil
	} else if tokenizer.s[tokenizer.head] == '(' {
		tokenizer.head++

		return Token{
			Kind: OpenParenthesis,
			Body: "(",
		}, nil
	} else if tokenizer.s[tokenizer.head] == ')' {
		tokenizer.head++

		return Token{
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

	var token Token
	switch body {
	case "pod":
		token = Token{
			Kind: Pod,
			Body: "pod",
		}
	case "namespace":
		token = Token{
			Kind: Namespace,
			Body: "namespace",
		}
	case "and", "or":
		token = Token{
			Kind: Operator,
			Body: body,
		}
	case "not":
		token = Token{
			Kind: Not,
			Body: "not",
		}
	default:
		token = Token{
			Kind: Pattern,
			Body: body,
		}
	}

	return token, nil
}

func (tokenizer *Tokenizer) Tokenize() ([]Token, error) {
	var tokens []Token

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
