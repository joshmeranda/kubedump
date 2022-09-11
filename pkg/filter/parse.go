package filter

import (
	"fmt"
	kubedump "kubedump/pkg"
	"strings"
)

func operatorPrecedence(op string) int {
	switch op {
	case "not":
		return 2
	case "and":
		return 1
	case "or":
		return 0
	default:
		return -1
	}
}

func reverseTokens(tokens []token) {
	for i, j := 0, len(tokens)-1; i < j; i, j = i+1, j-1 {
		tokens[i], tokens[j] = tokens[j], tokens[i]
	}
}

func prefixTokens(tokens []token) []token {
	reverseTokens(tokens)

	opStack := stack{}
	prefix := stack{}

	for _, t := range tokens {
		switch t.Kind {
		case Pattern, Resource, EOE:
			prefix.push(t)
		case Operator:
			if opStack.len() == 0 {
				opStack.push(t)
			} else if currentPrecedence, peekedPrecedence := operatorPrecedence(t.Body), operatorPrecedence(opStack.peek().Body); currentPrecedence >= peekedPrecedence {
				opStack.push(t)
			} else {
				for ; currentPrecedence < peekedPrecedence; peekedPrecedence = operatorPrecedence(opStack.peek().Body) {
					prefix.push(*opStack.pop())
				}
			}
		case OpenParenthesis:
			for t := opStack.pop(); t.Kind != CloseParenthesis; t = opStack.pop() {
				prefix.push(*t)
			}
		case CloseParenthesis:
			opStack.push(t)
		}
	}

	for opStack.len() > 0 {
		prefix.push(*opStack.pop())
	}

	innerPrefix := prefix.inner
	reverseTokens(innerPrefix)

	return innerPrefix
}

func splitPattern(pattern string) (string, string) {
	if pattern == "" {
		return "", ""
	}

	split := strings.SplitN(pattern, "/", 2)

	if len(split) == 1 {
		return "default", split[0]
	}

	return split[0], split[1]
}

type parser struct {
	tokens []token
	head   int
}

func (p *parser) parseExpression() (Expression, error) {
	switch p.tokens[p.head].Body {
	case "and", "or":
		return p.parseOperatorExpression()
	case "not":
		return p.parseNotExpression()
	case "pod", "job":
		return p.parseResourceExpression()
	}

	return nil, fmt.Errorf("unexpected token '%s'", p.tokens[p.head].Body)
}

func (p *parser) parseResourceExpression() (Expression, error) {
	kind := p.tokens[p.head].Body
	pattern := p.tokens[p.head+1].Body
	p.head += 2

	namespace, name := splitPattern(pattern)

	switch kubedump.ResourceKind(kind) {
	case "pod":
		return podExpression{
			NamePattern:      name,
			NamespacePattern: namespace,
		}, nil
	case "job":
		return jobExpression{
			NamePattern:      name,
			NamespacePattern: namespace,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported resource type '%s", kind)
	}
}

func (p *parser) parseOperatorExpression() (Expression, error) {
	operator := p.tokens[p.head]
	p.head++

	left, err := p.parseExpression()

	if err != nil {
		return nil, fmt.Errorf("could not parseExpression lhs: %w", err)
	}

	right, err := p.parseExpression()

	if err != nil {
		return nil, fmt.Errorf("could not parseExpression rhs: %w", err)
	}

	switch operator.Body {
	case "and":
		return andExpression{
			Left:  left,
			Right: right,
		}, nil
	case "or":
		return orExpression{
			Left:  left,
			Right: right,
		}, nil
	}

	return nil, fmt.Errorf("unsupported operator '%s'", operator.Body)
}

func (p *parser) parseNotExpression() (Expression, error) {
	p.head++
	expr, err := p.parseExpression()

	if err != nil {
		return nil, fmt.Errorf("could not parseExpression not expression: %w", err)
	}

	return notExpression{
		Inner: expr,
	}, nil
}

func Parse(s string) (Expression, error) {
	if s == "" {
		return truthyExpression{}, nil
	}

	tokenizer := newTokenizer(s)
	tokens, err := tokenizer.Tokenize()

	if err != nil {
		return nil, fmt.Errorf("could not tokenize: %w", err)
	}

	tokens = prefixTokens(tokens)

	p := parser{
		tokens: tokens,
	}

	expr, err := p.parseExpression()

	if err != nil {
		return nil, fmt.Errorf("could not parseExpression: %w", err)
	}

	return expr, nil
}
