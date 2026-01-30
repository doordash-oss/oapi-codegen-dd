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
	"testing"
)

func b(v bool) *bool { return &v }

func TestEncodeQueryFields_Arrays(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		enc      map[string]QueryEncoding
		expected string
	}{
		{
			name:     "form explode=true",
			data:     map[string]any{"expand": []string{"a", "b"}},
			enc:      map[string]QueryEncoding{"expand": {Style: "form", Explode: b(true)}},
			expected: "expand=a&expand=b",
		},
		{
			name:     "form explode=false",
			data:     map[string]any{"expand": []string{"a", "b"}},
			enc:      map[string]QueryEncoding{"expand": {Style: "form", Explode: b(false)}},
			expected: "expand=a,b", // Delimiter comma should NOT be encoded per OpenAPI spec
		},
		{
			name:     "form explode=false with comma in value",
			data:     map[string]any{"expand": []string{"a", "b", "c,d"}},
			enc:      map[string]QueryEncoding{"expand": {Style: "form", Explode: b(false)}},
			expected: "expand=a,b,c%2Cd", // Delimiter commas unescaped, comma in value escaped
		},
		{
			name:     "spaceDelimited",
			data:     map[string]any{"expand": []string{"a", "b"}},
			enc:      map[string]QueryEncoding{"expand": {Style: "spaceDelimited"}},
			expected: "expand=a%20b", // Space delimiter encoded as %20
		},
		{
			name:     "pipeDelimited",
			data:     map[string]any{"expand": []string{"a", "b"}},
			enc:      map[string]QueryEncoding{"expand": {Style: "pipeDelimited"}},
			expected: "expand=a%7Cb",
		},
		{
			name:     "deepObject array -> brackets",
			data:     map[string]any{"expand": []string{"a", "b"}},
			enc:      map[string]QueryEncoding{"expand": {Style: "deepObject"}},
			expected: "expand%5B%5D=a&expand%5B%5D=b",
		},
		{
			name:     "defaults (nil enc) form+explode=true",
			data:     map[string]any{"expand": []string{"a", "b"}},
			enc:      nil,
			expected: "expand=a&expand=b",
		},
		{
			name:     "deepObject scalar -> single bracketed",
			data:     map[string]any{"expand": "a"},
			enc:      map[string]QueryEncoding{"expand": {Style: "deepObject"}},
			expected: "expand%5B%5D=a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeQueryFields(tt.data, tt.enc)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tt.expected {
				t.Fatalf("%s: got %q, want %q", tt.name, got, tt.expected)
			}
		})
	}
}

func TestEncodeQueryFields_Objects(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		enc      map[string]QueryEncoding
		expected string
	}{
		{
			name:     "form explode=true",
			data:     map[string]any{"color": map[string]any{"R": "100", "G": "200", "B": "150"}},
			enc:      map[string]QueryEncoding{"color": {Style: "form", Explode: b(true)}},
			expected: "B=150&G=200&R=100",
		},
		{
			name:     "form explode=false",
			data:     map[string]any{"color": map[string]any{"R": "100", "G": "200", "B": "150"}},
			enc:      map[string]QueryEncoding{"color": {Style: "form", Explode: b(false)}},
			expected: "color=B,150,G,200,R,100", // Delimiter commas should NOT be encoded per OpenAPI spec
		},
		{
			name:     "spaceDelimited",
			data:     map[string]any{"color": map[string]any{"R": "100", "G": "200", "B": "150"}},
			enc:      map[string]QueryEncoding{"color": {Style: "spaceDelimited"}},
			expected: "color=B%20150%20G%20200%20R%20100", // Space delimiter encoded as %20
		},
		{
			name:     "pipeDelimited",
			data:     map[string]any{"color": map[string]any{"R": "100", "G": "200", "B": "150"}},
			enc:      map[string]QueryEncoding{"color": {Style: "pipeDelimited"}},
			expected: "color=B%7C150%7CG%7C200%7CR%7C100",
		},
		{
			name:     "deepObject (spec)",
			data:     map[string]any{"color": map[string]any{"R": "100", "G": "200", "B": "150"}},
			enc:      map[string]QueryEncoding{"color": {Style: "deepObject"}},
			expected: "color%5BB%5D=150&color%5BG%5D=200&color%5BR%5D=100",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeQueryFields(tt.data, tt.enc)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tt.expected {
				t.Fatalf("%s: got %q, want %q", tt.name, got, tt.expected)
			}
		})
	}
}

func TestEncodeQueryFields_ScalarsAndMultiParams(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		enc      map[string]QueryEncoding
		expected string
	}{
		{
			name:     "scalar default",
			data:     map[string]any{"x": "v"},
			enc:      nil,
			expected: "x=v",
		},
		{
			name: "multiple params sorted keys",
			data: map[string]any{
				"expand": []string{"a", "b"},
				"color":  map[string]any{"R": "100", "B": "150"},
				"x":      "v",
			},
			enc: map[string]QueryEncoding{
				"expand": {Style: "form", Explode: b(true)},
				"color":  {Style: "deepObject"},
			},
			expected: "color%5BB%5D=150&color%5BR%5D=100&expand=a&expand=b&x=v",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeQueryFields(tt.data, tt.enc)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tt.expected {
				t.Fatalf("%s: got %q, want %q", tt.name, got, tt.expected)
			}
		})
	}
}

func TestEncodeQueryFields_UnsupportedStyle(t *testing.T) {
	_, err := EncodeQueryFields(map[string]any{"a": []string{"x"}}, map[string]QueryEncoding{"a": {Style: "weird"}})
	if err == nil {
		t.Fatalf("expected error")
	}
}
