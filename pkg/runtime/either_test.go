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
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type nonZeroHook bool

func (h nonZeroHook) JSONNonZero() bool {
	return bool(h)
}

func TestNewEitherFromA(t *testing.T) {
	res := NewEitherFromA[string, int]("test")

	assert.True(t, res.IsA())
	assert.Equal(t, "test", res.A)
	assert.False(t, res.IsB())
	assert.Equal(t, 1, res.N)
	assert.Equal(t, 0, res.B)
}

func TestNewEitherFromB(t *testing.T) {
	res := NewEitherFromB[string, int](10)

	assert.False(t, res.IsA())
	assert.Equal(t, "", res.A)
	assert.True(t, res.IsB())
	assert.Equal(t, 2, res.N)
	assert.Equal(t, 10, res.B)
}

func TestEither_Value(t *testing.T) {
	res := NewEitherFromA[string, int]("test")
	assert.Equal(t, "test", res.Value())

	res = NewEitherFromB[string, int](10)
	assert.Equal(t, 10, res.Value())

	res = Either[string, int]{}
	assert.Nil(t, res.Value())
}

func TestEither_Unmarshal(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected Either[string, int]
	}{
		{
			name:     "string",
			input:    []byte(`"test"`),
			expected: NewEitherFromA[string, int]("test"),
		},
		{
			name:     "int",
			input:    []byte(`10`),
			expected: NewEitherFromB[string, int](10),
		},
		{
			name:     "null",
			input:    []byte(`null`),
			expected: Either[string, int]{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var res Either[string, int]
			err := res.UnmarshalJSON(test.input)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, res)
		})
	}
}

type NameOrID struct {
	Either[IDWrapper, NameWrapper]
}

type IDWrapper struct {
	ID int `json:"id"`
}

type NameWrapper struct {
	Name string `json:"name"`
}

func TestEither_MarshalJSON_with_wrapper(t *testing.T) {
	tests := []struct {
		name     string
		input    NameOrID
		expected []byte
	}{
		{
			name:     "id",
			input:    NameOrID{Either: NewEitherFromA[IDWrapper, NameWrapper](IDWrapper{ID: 10})},
			expected: []byte(`{"id":10}`),
		},
		{
			name:     "name",
			input:    NameOrID{Either: NewEitherFromB[IDWrapper, NameWrapper](NameWrapper{Name: "test"})},
			expected: []byte(`{"name":"test"}`),
		},
		{
			name:     "unset",
			input:    NameOrID{},
			expected: []byte(`null`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := test.input.MarshalJSON()
			assert.NoError(t, err)
			assert.JSONEq(t, string(test.expected), string(res))
		})
	}
}

func TestEither_UnmarshalJSON_with_wrapper(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected NameOrID
	}{
		{
			name:     "id",
			input:    []byte(`{"id":10}`),
			expected: NameOrID{Either: NewEitherFromA[IDWrapper, NameWrapper](IDWrapper{ID: 10})},
		},
		{
			name:     "name",
			input:    []byte(`{"name":"test"}`),
			expected: NameOrID{Either: NewEitherFromB[IDWrapper, NameWrapper](NameWrapper{Name: "test"})},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var res NameOrID
			err := res.UnmarshalJSON(test.input)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, res)
		})
	}
}

type ValidatableStruct struct {
	Name string `validate:"required"`
	Age  int    `validate:"required,min=1"`
}

func (v ValidatableStruct) Validate() error {
	if v.Name == "" {
		return assert.AnError
	}
	if v.Age < 1 {
		return assert.AnError
	}
	return nil
}

func TestEither_Validate(t *testing.T) {
	t.Run("validates A variant when active", func(t *testing.T) {
		valid := ValidatableStruct{Name: "John", Age: 30}
		either := NewEitherFromA[ValidatableStruct, string](valid)

		err := either.Validate()
		assert.NoError(t, err)
	})

	t.Run("validates B variant when active", func(t *testing.T) {
		either := NewEitherFromB[ValidatableStruct, string]("test")

		err := either.Validate()
		assert.NoError(t, err)
	})

	t.Run("fails validation for invalid A variant", func(t *testing.T) {
		invalid := ValidatableStruct{Name: "", Age: 0}
		either := NewEitherFromA[ValidatableStruct, string](invalid)

		err := either.Validate()
		assert.Error(t, err)
	})

	t.Run("does not validate inactive B variant", func(t *testing.T) {
		valid := ValidatableStruct{Name: "John", Age: 30}
		either := NewEitherFromA[ValidatableStruct, string](valid)
		// B is inactive and would be invalid if checked (empty string)

		err := either.Validate()
		assert.NoError(t, err) // Should pass because only A is validated
	})

	t.Run("fails validation for invalid B variant", func(t *testing.T) {
		either := NewEitherFromB[string, ValidatableStruct](ValidatableStruct{})
		assert.Error(t, either.Validate())
	})

	t.Run("passes A variant that is not a Validator", func(t *testing.T) {
		either := NewEitherFromA[string, ValidatableStruct]("test")
		assert.NoError(t, either.Validate())
	})

	t.Run("returns nil when neither is active", func(t *testing.T) {
		var either Either[ValidatableStruct, string]
		assert.NoError(t, either.Validate())
	})
}

// Test types for disambiguation
type PersonWithRequired struct {
	Name string `json:"name" validate:"required"`
	Age  int    `json:"age"`
}

func (p PersonWithRequired) Validate() error {
	if p.Name == "" {
		return assert.AnError
	}
	return nil
}

type PersonWithoutRequired struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func (p PersonWithoutRequired) Validate() error {
	// Always valid
	return nil
}

func TestEither_UnmarshalJSON_Disambiguation(t *testing.T) {
	t.Run("prefers type that validates when both unmarshal successfully", func(t *testing.T) {
		// Both types can unmarshal this JSON, but only PersonWithoutRequired validates
		data := []byte(`{"name":"","age":25}`)
		var either Either[PersonWithRequired, PersonWithoutRequired]

		err := either.UnmarshalJSON(data)
		assert.NoError(t, err)
		assert.True(t, either.IsB(), "should choose B (PersonWithoutRequired) because it validates")
		assert.Equal(t, "", either.B.Name)
		assert.Equal(t, 25, either.B.Age)
	})

	t.Run("chooses A when only A validates", func(t *testing.T) {
		data := []byte(`{"name":"John","age":30}`)
		var either Either[PersonWithRequired, PersonWithoutRequired]

		err := either.UnmarshalJSON(data)
		assert.NoError(t, err)
		// Both validate, but validation doesn't disambiguate, so falls back to heuristics
		// Both are non-zero, so defaults to A
		assert.True(t, either.IsA(), "should default to A when both validate")
	})

	t.Run("falls back to A when both validate", func(t *testing.T) {
		data := []byte(`{"name":"Alice","age":25}`)
		var either Either[PersonWithoutRequired, PersonWithoutRequired]

		err := either.UnmarshalJSON(data)
		assert.NoError(t, err)
		assert.True(t, either.IsA(), "should default to A when both types validate")
	})
}

type BooleanStruct struct {
	Enabled bool `json:"enabled"`
}

func (b BooleanStruct) Validate() error {
	return nil
}

type StringStruct struct {
	Value string `json:"value"`
}

func (s StringStruct) Validate() error {
	return nil
}

func TestEither_UnmarshalJSON_BooleanDisambiguation(t *testing.T) {
	t.Run("false boolean is non-zero and should be preferred", func(t *testing.T) {
		data := []byte(`{"enabled":false}`)
		var either Either[BooleanStruct, StringStruct]

		err := either.UnmarshalJSON(data)
		assert.NoError(t, err)
		// BooleanStruct unmarshals successfully with enabled=false
		// StringStruct unmarshals with zero value (empty string)
		// isNonZero(BooleanStruct{false}) should return true for the struct itself
		// This tests the heuristic when validation doesn't help
		assert.True(t, either.IsA(), "should choose BooleanStruct")
		assert.False(t, either.A.Enabled)
	})

	t.Run("true boolean is non-zero", func(t *testing.T) {
		data := []byte(`{"enabled":true}`)
		var either Either[BooleanStruct, StringStruct]

		err := either.UnmarshalJSON(data)
		assert.NoError(t, err)
		assert.True(t, either.IsA(), "should choose BooleanStruct")
		assert.True(t, either.A.Enabled)
	})
}

// Structs with nearly identical fields - only validation can disambiguate
type CreateUserRequest struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required"`
	Age   int    `json:"age"`
}

func (c CreateUserRequest) Validate() error {
	if c.Name == "" || c.Email == "" {
		return assert.AnError
	}
	return nil
}

type UpdateUserRequest struct {
	Name  string `json:"name"`  // optional
	Email string `json:"email"` // optional
	Age   int    `json:"age"`
}

func (u UpdateUserRequest) Validate() error {
	// All fields optional
	return nil
}

func TestEither_UnmarshalJSON_ValidationOnlyDisambiguation(t *testing.T) {
	t.Run("validation disambiguates when fields are identical but requirements differ", func(t *testing.T) {
		// Both structs have same fields, both unmarshal successfully
		// Only validation can tell them apart
		data := []byte(`{"name":"","email":"","age":25}`)
		var either Either[CreateUserRequest, UpdateUserRequest]

		err := either.UnmarshalJSON(data)
		assert.NoError(t, err)
		// CreateUserRequest fails validation (name and email required)
		// UpdateUserRequest passes validation (all optional)
		assert.True(t, either.IsB(), "should choose UpdateUserRequest because it validates")
		assert.Equal(t, "", either.B.Name)
		assert.Equal(t, "", either.B.Email)
		assert.Equal(t, 25, either.B.Age)
	})

	t.Run("chooses type that validates when both have data", func(t *testing.T) {
		// With valid data, CreateUserRequest validates
		data := []byte(`{"name":"John","email":"john@example.com","age":30}`)
		var either Either[CreateUserRequest, UpdateUserRequest]

		err := either.UnmarshalJSON(data)
		assert.NoError(t, err)
		// Both validate successfully, falls back to heuristics then A
		assert.True(t, either.IsA(), "should default to A when both validate")
		assert.Equal(t, "John", either.A.Name)
		assert.Equal(t, "john@example.com", either.A.Email)
		assert.Equal(t, 30, either.A.Age)
	})

	t.Run("partial data - only one validates", func(t *testing.T) {
		// Only name provided - CreateUserRequest fails (email required)
		data := []byte(`{"name":"Alice","age":28}`)
		var either Either[CreateUserRequest, UpdateUserRequest]

		err := either.UnmarshalJSON(data)
		assert.NoError(t, err)
		// CreateUserRequest fails validation (missing email)
		// UpdateUserRequest passes validation (email optional)
		assert.True(t, either.IsB(), "should choose UpdateUserRequest because it validates")
		assert.Equal(t, "Alice", either.B.Name)
		assert.Equal(t, "", either.B.Email)
		assert.Equal(t, 28, either.B.Age)
	})

	t.Run("prefers A when only A validates", func(t *testing.T) {
		data := []byte(`{"name":"","email":"","age":25}`)
		var either Either[UpdateUserRequest, CreateUserRequest]

		err := either.UnmarshalJSON(data)
		assert.NoError(t, err)
		assert.True(t, either.IsA(), "should choose UpdateUserRequest because it validates")
	})
}

func TestEither_UnmarshalJSON_FailsBoth(t *testing.T) {
	var either Either[string, int]
	assert.ErrorIs(t, either.UnmarshalJSON([]byte(`[1,2,3]`)), ErrFailedToUnmarshalAsAOrB)
}

func TestEither_FormMembers(t *testing.T) {
	assert.Equal(t, []reflect.Type{reflect.TypeFor[int64](), reflect.TypeFor[string]()}, new(Either[int64, string]).formMembers())
}

func TestIsNonZero(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{"nil", nil, false},
		{"JSONNonZero hook", nonZeroHook(true), true},
		{"IsZero hook", time.Time{}, false},
		{"bool", true, true},
		{"string", "", false},
		{"int", int64(0), false},
		{"uint", uint(0), false},
		{"float", 0.0, false},
		{"nil pointer", (*int)(nil), false},
		{"pointer", new(int), true},
		{"bytes", []byte{}, false},
		{"slice", []any{1}, true},
		{"map", map[string]any{}, false},
		{"struct", struct{ A int }{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isNonZero(tt.value))
		})
	}
}
