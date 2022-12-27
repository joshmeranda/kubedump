package filter

import (
	"fmt"
	"regexp"
	"strings"
)

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

// splitLabelPattern attempts to split the given label into the key and value pair it represents.
func splitLabelPattern(pattern string) (string, string, error) {
	if !strings.ContainsRune(pattern, '=') {
		return "", "", fmt.Errorf("label pattern does not contain an '='")
	}

	split := strings.SplitN(pattern, "=", 2)

	key, value := "", ""

	if len(split) == 1 {
		if strings.Index(pattern, "=") == 0 {
			return "", "", fmt.Errorf("key value cannot be empty")
		} else {
			key = split[0]
		}
	} else {
		key, value = split[0], split[1]
	}

	if err := validateLabelKey(key); err != nil {
		return "", "", fmt.Errorf("key '%s' is not valid: %w", key, err)
	}

	if err := validateLabelValue(value); err != nil {
		return "", "", fmt.Errorf("value '%s' is not valid: %w", value, err)
	}

	return key, value, nil
}

const (
	dnsLabelPatternFmt               = "[a-z0-9*]([a-z0-9\\-*]{0,61}[a-z0-9*])?"
	dnsSubdomainPatternFmt           = "[a-z0-9*]([a-z0-9\\-.*]{0,251}[a-z0-9*])?"
	dnsSubdomainPatternNoWildcardFmt = "[a-z0-9]([a-z0-9\\-.]{0,251}[a-z0-9])?"

	labelKeyPrefixPatternFmt = dnsSubdomainPatternNoWildcardFmt + "/"
	labelKeyNamePatternFmt   = "[a-zA-Z0-9]([a-zA-Z0-9\\-_.]{0,61}[a-zA-Z0-9])?"
	labelKeyPatternFmt       = "(" + labelKeyPrefixPatternFmt + ")?" + labelKeyNamePatternFmt

	labelValuePatternFmt = "([a-zA-Z0-9]([a-zA-Z0-9\\-_.]{0,61}[a-zA-Z0-9*])?)?"
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
	if !labelValuePattern.MatchString(value) {
		return fmt.Errorf("label valud '%s' is not valid", value)
	}

	return nil
}

func validateNamespace(namespace string) error {
	return validateDnsLabelRfc1123("namespace", namespace)
}

func validateResourceName(kind string, name string) error {
	return validateDnsSubdomain(kind, name)
}
