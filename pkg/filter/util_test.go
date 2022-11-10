package filter

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestPrefix(t *testing.T) {
	// not pod a and (pod b or pod c)
	tokens := []token{
		{
			Kind: Operator,
			Body: "not",
		},
		{
			Kind: Resource,
			Body: "pod",
		},
		{
			Kind: Pattern,
			Body: "a",
		},
		{
			Kind: Operator,
			Body: "and",
		},
		{
			Kind: OpenParenthesis,
			Body: "(",
		},
		{
			Kind: Resource,
			Body: "pod",
		},
		{
			Kind: Pattern,
			Body: "b",
		},
		{
			Kind: Operator,
			Body: "or",
		},
		{
			Kind: Resource,
			Body: "pod",
		},
		{
			Kind: Pattern,
			Body: "c",
		},
		{
			Kind: CloseParenthesis,
			Body: ")",
		},
		{
			Kind: EOE,
			Body: "EOE",
		},
	}

	// and not pod a or pod b pod c
	expected := []token{
		{
			Kind: Operator,
			Body: "and",
		},
		{
			Kind: Operator,
			Body: "not",
		},
		{
			Kind: Resource,
			Body: "pod",
		},
		{
			Kind: Pattern,
			Body: "a",
		},
		{
			Kind: Operator,
			Body: "or",
		},
		{
			Kind: Resource,
			Body: "pod",
		},
		{
			Kind: Pattern,
			Body: "b",
		},
		{
			Kind: Resource,
			Body: "pod",
		},
		{
			Kind: Pattern,
			Body: "c",
		},
		{
			Kind: EOE,
			Body: "EOE",
		},
	}
	actual := prefixTokens(tokens)

	assert.Equal(t, expected, actual)
}

func TestChainedPrefix(t *testing.T) {
	// pod a and pod b and pod c EOE
	tokens := []token{
		{
			Kind: Resource,
			Body: "pod",
		},
		{
			Kind: Pattern,
			Body: "a",
		},
		{
			Kind: Operator,
			Body: "and",
		},
		{
			Kind: Resource,
			Body: "pod",
		},
		{
			Kind: Pattern,
			Body: "b",
		},
		{
			Kind: Operator,
			Body: "and",
		},
		{
			Kind: Resource,
			Body: "pod",
		},
		{
			Kind: Pattern,
			Body: "c",
		},
		{
			Kind: EOE,
			Body: "EOE",
		},
	}

	// and and pod a pod b pod c
	actual := prefixTokens(tokens)
	expected := []token{
		{
			Kind: Operator,
			Body: "and",
		},
		{
			Kind: Operator,
			Body: "and",
		},
		{
			Kind: Resource,
			Body: "pod",
		},
		{
			Kind: Pattern,
			Body: "a",
		},
		{
			Kind: Resource,
			Body: "pod",
		},
		{
			Kind: Pattern,
			Body: "b",
		},
		{
			Kind: Resource,
			Body: "pod",
		},
		{
			Kind: Pattern,
			Body: "c",
		},
		{
			Kind: EOE,
			Body: "EOE",
		},
	}

	assert.Equal(t, expected, actual)
}

func TestSplitPattern(t *testing.T) {
	namespace, name := splitPattern("namespace/name/name")
	assert.Equal(t, "namespace", namespace)
	assert.Equal(t, "name/name", name)

	namespace, name = splitPattern("namespace/name")
	assert.Equal(t, "namespace", namespace)
	assert.Equal(t, "name", name)

	namespace, name = splitPattern("name")
	assert.Equal(t, "default", namespace)
	assert.Equal(t, "name", name)

	namespace, name = splitPattern("")
	assert.Zero(t, namespace)
	assert.Zero(t, name)
}

func TestSplitLabelPattern(t *testing.T) {
	assertLabelSplit := func(pattern string, key string, value string, found bool) {
		k, v, f := splitLabelPattern(pattern)

		assert.Equal(t, key, k, "bad key '%s' for pattern '%s'", k, pattern)
		assert.Equal(t, value, v, "bad value '%s' for pattern '%s'", v, pattern)
		assert.Equal(t, found, f, "bad found '%s' for pattern '%s'", found, pattern)
	}

	assertLabelSplit("=", "", "", true)
	assertLabelSplit("key=", "key", "", true)
	assertLabelSplit("=value", "", "value", true)
	assertLabelSplit("key=value", "key", "value", true)

	assertLabelSplit("no-label", "", "", false)
}

func TestValidateDnsSubdomain(t *testing.T) {
	assert.NoError(t, validateDnsSubdomain("test", "a"))
	assert.NoError(t, validateDnsSubdomain("test", "0"))
	assert.NoError(t, validateDnsSubdomain("test", "a-z"))
	assert.NoError(t, validateDnsSubdomain("test", "0-9"))
	assert.NoError(t, validateDnsSubdomain("test", "bilbo-baggins-of-the-shire"))
	assert.NoError(t, validateDnsSubdomain("test", "bilbo.baggins.of.the.shire"))

	assert.Error(t, validateDnsSubdomain("test", "a-"))
	assert.Error(t, validateDnsSubdomain("test", "A"))
	assert.Error(t, validateDnsSubdomain("test", "bilbo-baggins@shire"))

	assert.NoError(t, validateDnsSubdomain("test", strings.Repeat("a", 253)))
	assert.Error(t, validateDnsSubdomain("test", strings.Repeat("a", 254)))

	assert.NoError(t, validateDnsSubdomain("test", "subdomain.with.wildcard-*"))
}

func TestValidateDnsLabelRfc1123(t *testing.T) {
	assert.NoError(t, validateDnsLabelRfc1123("test", "a"))
	assert.NoError(t, validateDnsLabelRfc1123("test", "0"))
	assert.NoError(t, validateDnsLabelRfc1123("test", "a-z"))
	assert.NoError(t, validateDnsLabelRfc1123("test", "0-9"))
	assert.NoError(t, validateDnsLabelRfc1123("test", "bilbo-baggins-of-the-shire"))

	assert.Error(t, validateDnsLabelRfc1123("test", "a-"))
	assert.Error(t, validateDnsLabelRfc1123("test", "A"))
	assert.Error(t, validateDnsLabelRfc1123("test", "bilbo-baggins@shire"))

	assert.NoError(t, validateDnsLabelRfc1123("test", strings.Repeat("a", 63)))
	assert.Error(t, validateDnsLabelRfc1123("test", strings.Repeat("a", 64)))

	assert.NoError(t, validateDnsLabelRfc1123("test", "name-with-wildcard-*"))
}

func TestValidateDnsLabelRfc1035(t *testing.T) {
	assert.NoError(t, validateDnsLabelRfc1035("test", "a"))
	assert.NoError(t, validateDnsLabelRfc1035("test", "0"))
	assert.NoError(t, validateDnsLabelRfc1035("test", "a-z"))
	assert.NoError(t, validateDnsLabelRfc1035("test", "0-9"))
	assert.NoError(t, validateDnsLabelRfc1035("test", "bilbo-baggins-of-the-shire"))

	assert.Error(t, validateDnsLabelRfc1035("test", "a-"))
	assert.Error(t, validateDnsLabelRfc1035("test", "A"))
	assert.Error(t, validateDnsLabelRfc1035("test", "bilbo-baggins@shire"))

	assert.NoError(t, validateDnsLabelRfc1035("test", strings.Repeat("a", 63)))
	assert.Error(t, validateDnsLabelRfc1035("test", strings.Repeat("a", 64)))

	assert.NoError(t, validateDnsLabelRfc1035("test", "name-with-wildcard-*"))
}

func TestValidateLabelKey(t *testing.T) {
	assert.NoError(t, validateLabelKey("some.subdomain-0/some.label-name_0"))

	assert.NoError(t, validateLabelKey(strings.Repeat("a", 63)))
	assert.Error(t, validateLabelKey(strings.Repeat("a", 64)))

	assert.NoError(t, validateLabelKey(strings.Repeat("a", 253)+"/"+strings.Repeat("a", 63)))
	assert.Error(t, validateLabelKey(strings.Repeat("a", 254)+"/"+strings.Repeat("a", 63)))
	assert.Error(t, validateLabelKey(strings.Repeat("a", 253)+"/"+strings.Repeat("a", 64)))

	assert.NoError(t, validateLabelKey("prefix.with.wildcard-*/name.with.wildcard-*"))

	assert.Error(t, validateLabelKey(""))
}

func TestValidateLabelValue(t *testing.T) {
	assert.NoError(t, validateLabelValue(""))
	assert.NoError(t, validateLabelValue("some.label-value_0"))

	assert.NoError(t, validateLabelValue(strings.Repeat("a", 63)))
	assert.Error(t, validateLabelValue(strings.Repeat("a", 64)))

	assert.NoError(t, validateLabelValue("label.value.with.wildcard-*"))
}
