package runtime

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

const expectedMask = defaultMaskReplacement

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
	t.Run("nil pointer returns nil", func(t *testing.T) {
		var ptr *string
		result := MaskSensitivePointer(ptr, SensitiveDataConfig{Type: MaskTypeFull})
		assert.Nil(t, result)
	})

	t.Run("non-nil pointer delegates to MaskSensitiveValue", func(t *testing.T) {
		value := "secret"
		result := MaskSensitivePointer(&value, SensitiveDataConfig{Type: MaskTypeFull})
		assert.Equal(t, expectedMask, result)
	})
}

func TestSlogAttr(t *testing.T) {
	attr := SlogAttr("email", "user@example.com", SensitiveDataConfig{Type: MaskTypeFull})
	assert.Equal(t, "email", attr.Key)
	assert.Equal(t, expectedMask, attr.Value.String())
}

func TestSlogAttrPtr(t *testing.T) {
	t.Run("nil pointer returns empty string", func(t *testing.T) {
		var ptr *string
		attr := SlogAttrPtr("email", ptr, SensitiveDataConfig{Type: MaskTypeFull})
		assert.Equal(t, "email", attr.Key)
		assert.Equal(t, "", attr.Value.String())
	})

	t.Run("non-nil pointer delegates to SlogAttr", func(t *testing.T) {
		value := "user@example.com"
		attr := SlogAttrPtr("email", &value, SensitiveDataConfig{Type: MaskTypeFull})
		assert.Equal(t, "email", attr.Key)
		assert.Equal(t, expectedMask, attr.Value.String())
	})
}

// testUser simulates a generated type with sensitive data
type testUser struct {
	Name  string
	Email string
}

// Masked returns a copy with sensitive fields masked (simulates generated code)
func (u testUser) Masked() testUser {
	masked := u
	masked.Email = MaskSensitiveString(u.Email, SensitiveDataConfig{Type: MaskTypeFull})
	return masked
}

// LogValue implements slog.LogValuer (simulates generated code)
func (u testUser) LogValue() slog.Value {
	type plain testUser
	return slog.AnyValue(plain(u.Masked()))
}

func TestSlogLogValuerIntegration(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	user := testUser{Name: "John", Email: "john@example.com"}
	logger.Info("user created", "user", user)

	output := buf.String()
	assert.Contains(t, output, "Name:John")
	assert.Contains(t, output, "Email:"+expectedMask)
	assert.NotContains(t, output, "john@example.com")
}
