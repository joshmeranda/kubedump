package filter

import (
	"fmt"
)

var (
	unexpectedEOE = fmt.Errorf("unexpected end-of-expressions (EOE)")
)

func unexpectedTokenErr(t token) error {
	return fmt.Errorf("unexpected token '%s'", t.Body)
}

func couldNotParseErr(err error) error {
	return fmt.Errorf("could not parse expression: %w", err)
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
	case "pod", "job", "deployment", "replicaset", "service":
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

	if err := validateNamespace(namespace); err != nil {
		return nil, couldNotParseErr(err)
	}

	switch kind.Body {
	case "pod":
		if err := validatePodName(name); err != nil {
			return nil, couldNotParseErr(err)
		}

		return podExpression{
			namePattern:      name,
			namespacePattern: namespace,
		}, nil
	case "job":
		if err := validateJobName(name); err != nil {
			return nil, couldNotParseErr(err)
		}

		return jobExpression{
			namePattern:      name,
			namespacePattern: namespace,
		}, nil
	case "deployment":
		if err := validateDeploymentName(name); err != nil {
			return nil, couldNotParseErr(err)
		}

		return deploymentExpression{
			namePattern:      name,
			namespacePattern: namespace,
		}, nil
	case "replicaset":
		if err := validateReplicasetName(name); err != nil {
			return nil, couldNotParseErr(err)
		}

		return replicasetExpression{
			namePattern:      name,
			namespacePattern: namespace,
		}, nil
	case "service":
		if err := validateServiceName(name); err != nil {
			return nil, couldNotParseErr(err)
		}

		return serviceExpression{
			namePattern:      name,
			namespacePattern: namespace,
		}, nil
	default:
		return nil, couldNotParseErr(fmt.Errorf("unsupported resource type '%s", kind.Body))
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

	if err := validateNamespace(pattern.Body); err != nil {
		return nil, couldNotParseErr(err)
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

		if err := validateLabelKey(key); err != nil {
			return nil, couldNotParseErr(err)
		}

		if err := validateLabelValue(value); err != nil {
			return nil, couldNotParseErr(err)
		}

		if valid {
			if key == "" {
				return nil, fmt.Errorf("label key can not be empty")
			}
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

	if t := p.peekNextToken(0); t.Kind != EOE {
		return nil, unexpectedTokenErr(*t)
	}

	return expr, nil
}
