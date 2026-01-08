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

	"github.com/stretchr/testify/assert"
)

func TestEscapeGoString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "string with backslash",
			input:    `N\A`,
			expected: `N\\A`,
		},
		{
			name:     "string with newline",
			input:    "hello\nworld",
			expected: `hello\nworld`,
		},
		{
			name:     "string with tab",
			input:    "hello\tworld",
			expected: `hello\tworld`,
		},
		{
			name:     "string with quote",
			input:    `hello"world`,
			expected: `hello\"world`,
		},
		{
			name:     "string with backslash and quote",
			input:    `hello\"world`,
			expected: `hello\\\"world`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "string with multiple backslashes",
			input:    `C:\Users\test`,
			expected: `C:\\Users\\test`,
		},
		{
			name:     "path with trailing quote (loket.nl spec issue)",
			input:    `/providers/employers/conceptemployees/{conceptEmployeeId}/dossier"`,
			expected: `/providers/employers/conceptemployees/{conceptEmployeeId}/dossier\"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeGoString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
