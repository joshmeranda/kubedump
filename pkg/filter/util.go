package filter

import (
	"fmt"
	"regexp"
	"strings"
)

type stack struct {
	inner []token
}

func (s *stack) push(t token) {
	s.inner = append(s.inner, t)
}

// pop will return and remove the last element on the stack or nil if the stack is empty.
func (s *stack) pop() *token {
	if len(s.inner) == 0 {
		return nil
	}

	t := s.inner[len(s.inner)-1]

	s.inner = s.inner[:len(s.inner)-1]

	return &t
}

// peek will return the last element on the stack or nil if the stack is empty.
func (s *stack) peek() *token {
	if len(s.inner) == 0 {
		return nil
	}

	return &s.inner[len(s.inner)-1]
}

func (s *stack) len() int {
	return len(s.inner)
}

func (s stack) String() string {
	builder := strings.Builder{}

	for i, t := range s.inner {
		if i == len(s.inner)-1 {
			builder.WriteString(t.Body)
		} else {
			builder.WriteString(t.Body + " ")
		}
	}

	return builder.String()
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
		panic("unsupported operator '" + op + "'")
	}
}

func reverseTokens(tokens []token) {
	for i, j := 0, len(tokens)-1; i < j; i, j = i+1, j-1 {
		tokens[i], tokens[j] = tokens[j], tokens[i]
	}
}

func prefixTokens(tokens []token) []token {
	// todo: do we really need to reverse the tokens first
	reverseTokens(tokens)

	opStack := stack{}
	prefix := stack{}

	for _, t := range tokens {
		switch t.Kind {
		case Operator:
			peekedOperator := opStack.peek()

			if opStack.len() == 0 || peekedOperator.Kind == CloseParenthesis {
				opStack.push(t)
			} else {
				currentPrecendence := operatorPrecedence(t.Body)
				peekedPrecendence := operatorPrecedence(peekedOperator.Body)

				if currentPrecendence >= peekedPrecendence {
					opStack.push(t)
				} else if currentPrecendence < peekedPrecendence {
					for ; currentPrecendence < peekedPrecendence; peekedPrecendence = operatorPrecedence(opStack.peek().Body) {
						prefix.push(*opStack.pop())
					}
				}
			}
		case CloseParenthesis:
			opStack.push(t)
		case OpenParenthesis:
			for popped := opStack.pop(); popped != nil && popped.Kind != CloseParenthesis; popped = opStack.pop() {
				prefix.push(*popped)
			}
		default:
			prefix.push(t)
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

const (
	dnsLabelPatternFmt     = "[a-z0-9*]([a-z0-9\\-*]{0,61}[a-z0-9*])?"
	dnsSubdomainPatternFmt = "[a-z0-9*]([a-z0-9\\-.*]{0,251}[a-z0-9*])?"

	labelKeyPrefixPatterFmt = dnsSubdomainPatternFmt + "/"
	labelKeyNamePatternFmt  = "[a-zA-Z0-9*]([a-zA-Z0-9\\-_.*]{0,61}[a-zA-Z0-9*])?"
	labelKeyPatternFmt      = "(" + labelKeyPrefixPatterFmt + ")?" + labelKeyNamePatternFmt

	labelValuePatternFmt = "([a-zA-Z0-9*]([a-zA-Z0-9\\-_.*]{0,61}[a-zA-Z0-9*])?)?"
)

var (
	labelKeyPattern   = regexp.MustCompile("^" + labelKeyPatternFmt + "$")
	labelValuePattern = regexp.MustCompile("^" + labelValuePatternFmt + "$")

	dnsSubdomainPattern = regexp.MustCompile("^" + dnsSubdomainPatternFmt + "$")

	dnsLabelPattern        = regexp.MustCompile("^" + dnsLabelPatternFmt + "$")
	dnsLabelPatternRfc1123 = dnsLabelPattern
	dnsLabelPatternRfc1035 = dnsLabelPattern
)

func resourceNameInvalid(kind string, name string, pattern string) error {
	return fmt.Errorf("name '%s' is invalid for kind '%s'", name, kind)
}

// validateDnsSubdomain returns an error if the given name does not conform to the RFC 1123 spec as defined here: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names
func validateDnsSubdomain(kind string, name string) error {
	if !dnsSubdomainPattern.MatchString(name) {
		return resourceNameInvalid(kind, name, dnsSubdomainPattern.String())
	}

	return nil
}

// validateDnsLabelRfc1123 returns an error if the given name does not conform to the RFC 1123 spec as specified here: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names
func validateDnsLabelRfc1123(kind string, label string) error {
	if !dnsLabelPatternRfc1123.MatchString(label) {
		return resourceNameInvalid(kind, label, dnsLabelPatternRfc1123.String())
	}

	return nil
}

// validateDnsLabelRfc1035 returns an error if the given name does not conform to the RFC 1035 spec as specified here: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#rfc-1035-label-names
func validateDnsLabelRfc1035(kind string, label string) error {
	if !dnsLabelPatternRfc1035.MatchString(label) {
		return resourceNameInvalid(kind, label, dnsLabelPatternRfc1035.String())
	}

	return nil
}

func validateLabelKey(key string) error {
	if !labelKeyPattern.MatchString(key) {
		return fmt.Errorf("label key '%s' is not valid", key)
	}

	return nil
}

func validateLabelValue(value string) error {
	// todo: ideally this would be handled by the regex, but this is fine for now
	//if value == "" {
	//	return nil
	//}

	if !labelValuePattern.MatchString(value) {
		return fmt.Errorf("label valud '%s' is not valid", value)
	}

	return nil
}

func validateNamespace(namespace string) error {
	return validateDnsLabelRfc1123("namespace", namespace)
}

func validatePodName(name string) error {
	return validateDnsSubdomain("pod", name)
}

func validateJobName(name string) error {
	return validateDnsSubdomain("job", name)
}

func validateDeploymentName(name string) error {
	return validateDnsSubdomain("job", name)
}

func validateReplicasetName(name string) error {
	return validateDnsSubdomain("replicaset", name)
}

func validateServiceName(name string) error {
	return validateDnsSubdomain("replicaset", name)
}
