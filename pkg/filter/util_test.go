package filter

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

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
}

func TestValidateLabelKey(t *testing.T) {
	assert.NoError(t, validateLabelKey("some.subdomain-0/some.label-value_0"))

	assert.NoError(t, validateLabelKey(strings.Repeat("a", 63)))
	assert.Error(t, validateLabelKey(strings.Repeat("a", 64)))

	assert.NoError(t, validateLabelKey(strings.Repeat("a", 253)+"/"+strings.Repeat("a", 63)))
	assert.Error(t, validateLabelKey(strings.Repeat("a", 254)+"/"+strings.Repeat("a", 63)))
	assert.Error(t, validateLabelKey(strings.Repeat("a", 253)+"/"+strings.Repeat("a", 64)))
}

func TestValidateLabelValue(t *testing.T) {
	assert.NoError(t, validateLabelKey("some.label-value_0"))

	assert.NoError(t, validateLabelKey(strings.Repeat("a", 63)))
	assert.Error(t, validateLabelKey(strings.Repeat("a", 64)))

	assert.NoError(t, validateLabelKey(strings.Repeat("a", 253)+"/"+strings.Repeat("a", 63)))
	assert.Error(t, validateLabelKey(strings.Repeat("a", 254)+"/"+strings.Repeat("a", 63)))
	assert.Error(t, validateLabelKey(strings.Repeat("a", 253)+"/"+strings.Repeat("a", 64)))
}
