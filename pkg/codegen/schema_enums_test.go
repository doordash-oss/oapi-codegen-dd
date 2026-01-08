// Copyright 2025 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package codegen

import (
	"testing"
)

func TestNeedsQuotesForEnumValue(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		goType   string
		expected bool
	}{
		{
			name:     "string type always needs quotes",
			value:    "test",
			goType:   "string",
			expected: true,
		},
		{
			name:     "numeric value with int type doesn't need quotes",
			value:    "123",
			goType:   "int",
			expected: false,
		},
		{
			name:     "string value with int type needs quotes",
			value:    "asc",
			goType:   "int",
			expected: true,
		},
		{
			name:     "string value with int32 type needs quotes",
			value:    "desc",
			goType:   "int32",
			expected: true,
		},
		{
			name:     "numeric value with float32 type doesn't need quotes",
			value:    "1.5",
			goType:   "float32",
			expected: false,
		},
		{
			name:     "string value with float64 type needs quotes",
			value:    "high",
			goType:   "float64",
			expected: true,
		},
		{
			name:     "boolean value with bool type doesn't need quotes",
			value:    "true",
			goType:   "bool",
			expected: false,
		},
		{
			name:     "string value with bool type needs quotes",
			value:    "yes",
			goType:   "bool",
			expected: true,
		},
		{
			name:     "negative number with int type doesn't need quotes",
			value:    "-42",
			goType:   "int",
			expected: false,
		},
		{
			name:     "decimal number with int type needs quotes (not a valid int)",
			value:    "1.5",
			goType:   "int",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := needsQuotesForEnumValue(tt.value, tt.goType)
			if result != tt.expected {
				t.Errorf("needsQuotesForEnumValue(%q, %q) = %v, want %v", tt.value, tt.goType, result, tt.expected)
			}
		})
	}
}
