// Copyright 2025 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v4"
)

const defaultMaskReplacement = "********"

// MaskType represents the type of masking to apply
type MaskType string

// Masking type constants
const (
	MaskTypeFull    MaskType = "full"
	MaskTypeRegex   MaskType = "regex"
	MaskTypeHash    MaskType = "hash"
	MaskTypePartial MaskType = "partial"
)

// SensitiveDataConfig holds configuration for masking sensitive data
type SensitiveDataConfig struct {
	Type        MaskType // masking type: full, regex, hash, or partial
	Replacement string   // custom replacement string for "full" and "partial" masks (default: "********")
	Pattern     string   // regex pattern for "regex" type
	Algorithm   string   // hash algorithm for "hash" type (e.g., "sha256")
	KeepPrefix  int      // number of characters to keep at start for "partial" type
	KeepSuffix  int      // number of characters to keep at end for "partial" type
}

// NewDefaultSensitiveDataConfig returns a SensitiveDataConfig with default settings (full masking)
func NewDefaultSensitiveDataConfig() *SensitiveDataConfig {
	return &SensitiveDataConfig{
		Type: MaskTypeFull,
	}
}

// sensitiveDataYAML is a helper struct for unmarshaling the x-sensitive-data extension
type sensitiveDataYAML struct {
	Mask       string `yaml:"mask" json:"mask"`
	Pattern    string `yaml:"pattern" json:"pattern"`
	Algorithm  string `yaml:"algorithm" json:"algorithm"`
	KeepPrefix int    `yaml:"keepPrefix" json:"keepPrefix"`
	KeepSuffix int    `yaml:"keepSuffix" json:"keepSuffix"`
}

// Unmarshal parses the x-sensitive-data extension value from YAML/JSON
// Supports:
// - boolean: true -> full masking
// - string: "full", "hash", "regex", "partial" -> that masking type
// - object: detailed configuration with mask type and parameters
func (s *SensitiveDataConfig) Unmarshal(value any) error {
	// Handle simple boolean value (defaults to "full" masking)
	if b, ok := value.(bool); ok {
		if b {
			s.Type = MaskTypeFull
		}
		return nil
	}

	// Handle simple string value
	if str, ok := value.(string); ok {
		s.Type = MaskType(str)
		return nil
	}

	// Handle object with detailed configuration - marshal to YAML and unmarshal to struct
	yamlBytes, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal x-sensitive-data value: %w", err)
	}

	var helper sensitiveDataYAML
	helper.Mask = "full" // default

	if err := yaml.Unmarshal(yamlBytes, &helper); err != nil {
		return fmt.Errorf("failed to unmarshal x-sensitive-data: %w", err)
	}

	// Populate the config
	s.Type = MaskType(helper.Mask)
	s.Pattern = helper.Pattern
	s.Algorithm = helper.Algorithm
	s.KeepPrefix = helper.KeepPrefix
	s.KeepSuffix = helper.KeepSuffix

	return nil
}

// Mask returns the Type as a string for template compatibility
func (s *SensitiveDataConfig) Mask() string {
	return string(s.Type)
}

// EscapedPattern returns the pattern with backslashes escaped for use in Go string literals
func (s *SensitiveDataConfig) EscapedPattern() string {
	return strings.ReplaceAll(s.Pattern, `\`, `\\`)
}

// MaskSensitivePointer masks a pointer value
func MaskSensitivePointer[T any](value *T, config SensitiveDataConfig) any {
	if value == nil {
		return nil
	}
	return MaskSensitiveValue(*value, config)
}

// MaskSensitiveValue masks a sensitive value based on the masking strategy
func MaskSensitiveValue(value any, config SensitiveDataConfig) any {
	if value == nil {
		return nil
	}

	// Convert value to string for masking
	strValue := fmt.Sprintf("%v", value)

	// Get replacement string (use default if not specified)
	replacement := config.Replacement
	if replacement == "" {
		replacement = defaultMaskReplacement
	}

	switch config.Type {
	case MaskTypeFull:
		return maskFull(strValue, replacement)
	case MaskTypeRegex:
		if config.Pattern == "" {
			return maskFull(strValue, replacement)
		}
		return maskRegex(strValue, config.Pattern)
	case MaskTypeHash:
		algorithm := config.Algorithm
		if algorithm == "" {
			algorithm = "sha256"
		}
		return maskHash(strValue, algorithm)
	case MaskTypePartial:
		return maskPartial(strValue, replacement, config.KeepPrefix, config.KeepSuffix)
	default:
		// Default to full masking
		return maskFull(strValue, replacement)
	}
}

// MaskSensitiveString is a convenience function for masking string values
func MaskSensitiveString(value string, config SensitiveDataConfig) string {
	result := MaskSensitiveValue(value, config)
	if result == nil {
		return ""
	}
	return fmt.Sprintf("%v", result)
}

// SlogAttr creates a slog.Attr with a masked value for sensitive data.
// This is used by generated LogValue() methods to mask sensitive fields in structured logging.
func SlogAttr[T any](key string, value T, config SensitiveDataConfig) slog.Attr {
	masked := MaskSensitiveString(fmt.Sprintf("%v", value), config)
	return slog.String(key, masked)
}

// SlogAttrPtr creates a slog.Attr with a masked value for sensitive pointer data.
// Returns an empty string attribute if the pointer is nil.
func SlogAttrPtr[T any](key string, value *T, config SensitiveDataConfig) slog.Attr {
	if value == nil {
		return slog.String(key, "")
	}
	return SlogAttr(key, *value, config)
}

// maskFull replaces the entire value with a fixed mask to hide the length
func maskFull(value, replacement string) string {
	if len(value) == 0 {
		return ""
	}
	return replacement
}

// maskRegex masks parts of the value matching the regex pattern
func maskRegex(value, pattern string) string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		// If regex is invalid, fall back to full masking
		return maskFull(value, defaultMaskReplacement)
	}

	return re.ReplaceAllStringFunc(value, func(match string) string {
		return strings.Repeat("*", len(match))
	})
}

// maskHash returns a hash of the value
func maskHash(value, algorithm string) string {
	switch algorithm {
	case "sha256":
		hash := sha256.Sum256([]byte(value))
		return hex.EncodeToString(hash[:])
	default:
		// Default to sha256
		hash := sha256.Sum256([]byte(value))
		return hex.EncodeToString(hash[:])
	}
}

// maskPartial masks the middle part of a value, keeping prefix and suffix visible
func maskPartial(value, replacement string, keepPrefix, keepSuffix int) string {
	if len(value) == 0 {
		return ""
	}

	// If the value is too short to partially mask, use full masking
	if len(value) <= keepPrefix+keepSuffix {
		return maskFull(value, replacement)
	}

	prefix := ""
	suffix := ""

	if keepPrefix > 0 {
		prefix = value[:keepPrefix]
	}

	if keepSuffix > 0 {
		suffix = value[len(value)-keepSuffix:]
	}

	return prefix + replacement + suffix
}
