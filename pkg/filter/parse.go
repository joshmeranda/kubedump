package filter

import (
	"fmt"
	"strings"
)

var (
	unexpectedEOE = fmt.Errorf("unexpected end-of-expressions (EOE)")
)

func unexpectedTokenErr(t token) error {
	return fmt.Errorf("unexpected token '%s'", t.Body)
}

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
		case Pattern, Resource, Namespace, Label, EOE:
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

func splitLabelPattern(pattern string) (string, string, bool) {
	if !strings.ContainsRune(pattern, '=') {
		return "", "", false
	}

	split := strings.SplitN(pattern, "=", 2)

	if len(split) == 1 {
		if strings.Index(pattern, "=") == 0 {
			return "", split[0], true
		} else {
			return split[0], "", true
		}
	}

	return split[0], split[1], true
}

type parser struct {
	tokens []token
	head   int
}

// nextToken returns the next token in the parser, or nil if there are non lefts
func (p *parser) nextToken() *token {
	if p.head >= len(p.tokens) {
		return nil
	}

	p.head++
	return &p.tokens[p.head-1]
}

func (p *parser) peekNextToken(offset int) *token {
	if p.head+offset >= len(p.tokens) {
		return nil
	}

	return &p.tokens[p.head+offset]
}

func (p *parser) parseExpression() (Expression, error) {
	// todo: we should replace this with `p.nextToken`
	switch p.tokens[p.head].Body {
	case "and", "or":
		return p.parseOperatorExpression()
	case "not":
		return p.parseNotExpression()
	case "pod", "job":
		return p.parseResourceExpression()
	case "namespace":
		return p.parseNamespaceExpression()
	case "label":
		return p.parseLabelExpression()
	}

	return nil, unexpectedTokenErr(p.tokens[p.head])
}

func (p *parser) parseResourceExpression() (Expression, error) {
	kind := p.tokens[p.head]
	pattern := p.tokens[p.head+1]
	p.head += 2

	if pattern.Kind == EOE {
		return nil, unexpectedEOE
	}

	namespace, name := splitPattern(pattern.Body)

	switch kind.Body {
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
		return nil, fmt.Errorf("unsupported resource type '%s", kind.Body)
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

func (p *parser) parseNamespaceExpression() (Expression, error) {
	pattern := p.tokens[p.head+1]
	p.head += 2

	if pattern.Kind == EOE {
		return nil, unexpectedEOE
	}

	return namespaceExpression{
		NamespacePattern: pattern.Body,
	}, nil
}

func (p *parser) parseLabelExpression() (Expression, error) {
	p.nextToken()
	labels := make(map[string]string)

	for {
		t := p.peekNextToken(0)

		if t.Kind == EOE {
			break
		}

		key, value, valid := splitLabelPattern(t.Body)

		if valid {
			labels[key] = value
			p.nextToken()
		} else {
			break
		}
	}

	return labelExpression{
		labelPatterns: labels,
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
