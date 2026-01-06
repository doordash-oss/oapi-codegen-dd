package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Use the same constant as the implementation for consistency
const expectedMask = defaultMaskReplacement

func TestMaskFull(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "single character",
			input:    "a",
			expected: expectedMask,
		},
		{
			name:     "three characters",
			input:    "abc",
			expected: expectedMask,
		},
		{
			name:     "four characters",
			input:    "abcd",
			expected: expectedMask,
		},
		{
			name:     "email",
			input:    "user@example.com",
			expected: expectedMask,
		},
		{
			name:     "long string",
			input:    "this is a very long secret string",
			expected: expectedMask,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskFull(tt.input, defaultMaskReplacement)
			assert.Equal(t, tt.expected, result)
			// All non-empty values should return the same fixed mask
			if tt.input != "" {
				assert.Equal(t, defaultMaskReplacement, result)
			}
		})
	}
}

func TestMaskRegex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		pattern  string
		expected string
	}{
		{
			name:     "SSN pattern",
			input:    "123-45-6789",
			pattern:  `\d{3}-\d{2}-\d{4}`,
			expected: "***********",
		},
		{
			name:     "credit card pattern",
			input:    "1234-5678-9012-3456",
			pattern:  `\d{4}-\d{4}-\d{4}-\d{4}`,
			expected: "*******************",
		},
		{
			name:     "partial match - digits only",
			input:    "My SSN is 123-45-6789 and my phone is 555-1234",
			pattern:  `\d+`,
			expected: "My SSN is ***-**-**** and my phone is ***-****",
		},
		{
			name:     "no match",
			input:    "no numbers here",
			pattern:  `\d+`,
			expected: "no numbers here",
		},
		{
			name:     "invalid regex - fallback to full masking",
			input:    "test@example.com",
			pattern:  `[invalid(`,
			expected: expectedMask,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskRegex(tt.input, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMaskHash(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		algorithm string
		expected  string
	}{
		{
			name:      "SHA256 hash",
			input:     "my-secret-api-key",
			algorithm: "sha256",
			expected:  "325ededd6c3b9988f623c7f964abb9b016b76b0f8b3474df0f7d7c23b941381f",
		},
		{
			name:      "empty string",
			input:     "",
			algorithm: "sha256",
			expected:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:      "different input produces different hash",
			input:     "different-key",
			algorithm: "sha256",
			expected:  "b421364fb76983fca05d031e1bda64bbd1618848962e81836495a5c57223a37e",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskHash(tt.input, tt.algorithm)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, 64, len(result)) // SHA256 produces 64 hex characters
		})
	}
}

func TestMaskSensitiveValue(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		config   SensitiveDataConfig
		expected any
	}{
		{
			name:     "nil value",
			value:    nil,
			config:   SensitiveDataConfig{Type: MaskTypeFull},
			expected: nil,
		},
		{
			name:     "full masking",
			value:    "secret",
			config:   SensitiveDataConfig{Type: MaskTypeFull},
			expected: expectedMask,
		},
		{
			name:     "regex masking",
			value:    "123-45-6789",
			config:   SensitiveDataConfig{Type: MaskTypeRegex, Pattern: `\d{3}-\d{2}-\d{4}`},
			expected: "***********",
		},
		{
			name:     "hash masking",
			value:    "api-key",
			config:   SensitiveDataConfig{Type: MaskTypeHash, Algorithm: "sha256"},
			expected: "8c284055dbb54b7f053a2dc612c3727c7aa36354361055f2110f4903ea8ee29c",
		},
		{
			name:     "regex with empty pattern falls back to full",
			value:    "secret",
			config:   SensitiveDataConfig{Type: MaskTypeRegex, Pattern: ""},
			expected: expectedMask,
		},
		{
			name:     "hash with empty algorithm defaults to sha256",
			value:    "key",
			config:   SensitiveDataConfig{Type: MaskTypeHash, Algorithm: ""},
			expected: "2c70e12b7a0646f92279f427c7b38e7334d8e5389cff167a1dc30e73f826b683",
		},
		{
			name:     "unknown mask type defaults to full",
			value:    "secret",
			config:   SensitiveDataConfig{Type: "unknown"},
			expected: expectedMask,
		},
		{
			name:     "integer value",
			value:    12345,
			config:   SensitiveDataConfig{Type: MaskTypeFull},
			expected: expectedMask,
		},
		{
			name:     "partial masking - credit card last 4",
			value:    "1234-5678-9012-3456",
			config:   SensitiveDataConfig{Type: MaskTypePartial, KeepSuffix: 4},
			expected: expectedMask + "3456",
		},
		{
			name:     "partial masking - keep prefix and suffix",
			value:    "1234567890",
			config:   SensitiveDataConfig{Type: MaskTypePartial, KeepPrefix: 2, KeepSuffix: 2},
			expected: "12" + expectedMask + "90",
		},
		{
			name:     "partial masking - value too short",
			value:    "abc",
			config:   SensitiveDataConfig{Type: MaskTypePartial, KeepPrefix: 2, KeepSuffix: 2},
			expected: expectedMask,
		},
		{
			name:     "custom replacement - full",
			value:    "secret",
			config:   SensitiveDataConfig{Type: MaskTypeFull, Replacement: "[REDACTED]"},
			expected: "[REDACTED]",
		},
		{
			name:     "custom replacement - partial",
			value:    "1234567890",
			config:   SensitiveDataConfig{Type: MaskTypePartial, Replacement: "***", KeepPrefix: 2, KeepSuffix: 2},
			expected: "12***90",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskSensitiveValue(tt.value, tt.config)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMaskSensitivePointer(t *testing.T) {
	t.Run("nil pointer", func(t *testing.T) {
		var ptr *string
		result := MaskSensitivePointer(ptr, SensitiveDataConfig{Type: MaskTypeFull})
		assert.Nil(t, result)
	})

	t.Run("non-nil pointer", func(t *testing.T) {
		value := "secret"
		result := MaskSensitivePointer(&value, SensitiveDataConfig{Type: MaskTypeFull})
		assert.Equal(t, expectedMask, result)
	})

	t.Run("pointer with regex masking", func(t *testing.T) {
		value := "123-45-6789"
		result := MaskSensitivePointer(&value, SensitiveDataConfig{Type: MaskTypeRegex, Pattern: `\d+`})
		assert.Equal(t, "***-**-****", result)
	})

	t.Run("pointer with partial masking", func(t *testing.T) {
		value := "1234567890"
		result := MaskSensitivePointer(&value, SensitiveDataConfig{Type: MaskTypePartial, KeepPrefix: 2, KeepSuffix: 2})
		assert.Equal(t, "12"+expectedMask+"90", result)
	})
}

func TestMaskSensitiveString(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		config   SensitiveDataConfig
		expected string
	}{
		{
			name:     "empty string",
			value:    "",
			config:   SensitiveDataConfig{Type: MaskTypeFull},
			expected: "",
		},
		{
			name:     "full masking",
			value:    "password123",
			config:   SensitiveDataConfig{Type: MaskTypeFull},
			expected: expectedMask,
		},
		{
			name:     "regex masking",
			value:    "card: 1234-5678-9012-3456",
			config:   SensitiveDataConfig{Type: MaskTypeRegex, Pattern: `\d{4}`},
			expected: "card: ****-****-****-****",
		},
		{
			name:     "partial masking",
			value:    "sensitive-data",
			config:   SensitiveDataConfig{Type: MaskTypePartial, KeepPrefix: 3, KeepSuffix: 4},
			expected: "sen" + expectedMask + "data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskSensitiveString(tt.value, tt.config)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMaskPartial(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		keepPrefix int
		keepSuffix int
		expected   string
	}{
		{
			name:       "empty string",
			input:      "",
			keepPrefix: 0,
			keepSuffix: 0,
			expected:   "",
		},
		{
			name:       "keep last 4 digits (credit card)",
			input:      "1234567890123456",
			keepPrefix: 0,
			keepSuffix: 4,
			expected:   expectedMask + "3456",
		},
		{
			name:       "keep first 3 and last 4",
			input:      "1234567890",
			keepPrefix: 3,
			keepSuffix: 4,
			expected:   "123" + expectedMask + "7890",
		},
		{
			name:       "value too short - falls back to full mask",
			input:      "abc",
			keepPrefix: 2,
			keepSuffix: 2,
			expected:   expectedMask,
		},
		{
			name:       "exact length match - falls back to full mask",
			input:      "abcd",
			keepPrefix: 2,
			keepSuffix: 2,
			expected:   expectedMask,
		},
		{
			name:       "no prefix or suffix",
			input:      "secret",
			keepPrefix: 0,
			keepSuffix: 0,
			expected:   expectedMask,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskPartial(tt.input, defaultMaskReplacement, tt.keepPrefix, tt.keepSuffix)
			assert.Equal(t, tt.expected, result)
		})
	}
}
