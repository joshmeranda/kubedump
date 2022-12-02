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

// splitLabelPattern attempts to split the given label into the key and value pair it represents, or empty strings and
// false if the pair could not be determined.
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
