// Copyright 2025 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runtime

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterCustomTypeFunc_WithEither(t *testing.T) {
	v := validator.New(validator.WithRequiredStructEnabled())
	RegisterCustomTypeFunc(v)

	type TestStruct struct {
		Value Either[string, int] `validate:"required"`
	}

	t.Run("validates active A variant", func(t *testing.T) {
		ts := TestStruct{
			Value: NewEitherFromA[string, int]("hello"),
		}
		err := v.Struct(ts)
		assert.NoError(t, err)
	})

	t.Run("validates active B variant", func(t *testing.T) {
		ts := TestStruct{
			Value: NewEitherFromB[string, int](42),
		}
		err := v.Struct(ts)
		assert.NoError(t, err)
	})

	t.Run("fails validation when no variant is active", func(t *testing.T) {
		ts := TestStruct{
			Value: Either[string, int]{}, // N=0, no active variant
		}
		err := v.Struct(ts)
		assert.Error(t, err)
	})
}

func TestRegisterCustomTypeFunc_WithValidateVar(t *testing.T) {
	v := validator.New(validator.WithRequiredStructEnabled())
	RegisterCustomTypeFunc(v)

	t.Run("validates active A variant with Var", func(t *testing.T) {
		either := NewEitherFromA[string, int]("hello")
		err := v.Var(either, "required")
		assert.NoError(t, err)
	})

	t.Run("validates active B variant with Var", func(t *testing.T) {
		either := NewEitherFromB[string, int](42)
		err := v.Var(either, "required")
		assert.NoError(t, err)
	})

	t.Run("returns nil when no variant is active with Var", func(t *testing.T) {
		either := Either[string, int]{} // N=0, no active variant

		// The custom type function returns nil for inactive variants,
		// which the validator treats as a zero value, not a validation error
		err := v.Var(either, "required")

		// This doesn't error because the validator sees nil from Value()
		// Actual validation of inactive variants should be done via Validate() method
		assert.NoError(t, err)
	})
}

func TestConvertValidatorError(t *testing.T) {
	v := validator.New(validator.WithRequiredStructEnabled())

	t.Run("returns nil for nil error", func(t *testing.T) {
		result := ConvertValidatorError(nil)
		assert.Nil(t, result)
	})

	t.Run("converts ValidationError to ValidationErrors", func(t *testing.T) {
		ve := NewValidationError("field", "message")
		result := ConvertValidatorError(ve)

		// Should be converted to ValidationErrors for consistency
		ves, ok := result.(ValidationErrors)
		require.True(t, ok, "expected ValidationErrors")
		require.Len(t, ves, 1)
		assert.Equal(t, "field", ves[0].Field)
		assert.Equal(t, "message", ves[0].Message)
	})

	t.Run("returns ValidationErrors as-is", func(t *testing.T) {
		ves := ValidationErrors{
			NewValidationError("field1", "message1"),
			NewValidationError("field2", "message2"),
		}
		result := ConvertValidatorError(ves)
		assert.Equal(t, ves, result)
	})

	t.Run("converts wrapped ValidationError to ValidationErrors", func(t *testing.T) {
		ve := NewValidationError("field", "message")
		wrapped := fmt.Errorf("wrapped: %w", ve)

		result := ConvertValidatorError(wrapped)

		// Should be converted to ValidationErrors for consistency
		ves, ok := result.(ValidationErrors)
		require.True(t, ok, "expected ValidationErrors")
		require.Len(t, ves, 1)
		// The wrapped error message becomes the message
		assert.Contains(t, ves[0].Message, "wrapped")
		assert.Contains(t, ves[0].Message, "field")
		assert.Contains(t, ves[0].Message, "message")
	})

	t.Run("handles wrapped ValidationErrors", func(t *testing.T) {
		ves := ValidationErrors{
			NewValidationError("field1", "message1"),
		}
		wrapped := fmt.Errorf("wrapped: %w", ves)

		result := ConvertValidatorError(wrapped)

		assert.Equal(t, wrapped, result)

		var unwrapped ValidationErrors
		assert.True(t, errors.As(result, &unwrapped))
		assert.Len(t, unwrapped, 1)
	})

	t.Run("converts validator.ValidationErrors", func(t *testing.T) {
		type TestStruct struct {
			Name  string `validate:"required"`
			Email string `validate:"required,email"`
		}

		ts := TestStruct{}
		err := v.Struct(ts)
		require.Error(t, err)

		result := ConvertValidatorError(err)

		var ves ValidationErrors
		require.True(t, errors.As(result, &ves), "expected ValidationErrors type")
		assert.NotEmpty(t, ves)
	})

	t.Run("converts wrapped validator.ValidationErrors", func(t *testing.T) {
		type TestStruct struct {
			Name string `validate:"required"`
		}

		ts := TestStruct{}
		validatorErr := v.Struct(ts)
		require.Error(t, validatorErr)

		wrapped := fmt.Errorf("validation failed: %w", validatorErr)

		result := ConvertValidatorError(wrapped)

		var ves ValidationErrors
		require.True(t, errors.As(result, &ves), "expected ValidationErrors type")
		assert.NotEmpty(t, ves)
	})

	t.Run("converts generic error", func(t *testing.T) {
		genericErr := errors.New("some error")

		result := ConvertValidatorError(genericErr)

		var ves ValidationErrors
		require.True(t, errors.As(result, &ves), "expected ValidationErrors type")
		assert.Len(t, ves, 1, "generic errors are now wrapped in ValidationError")
		assert.Equal(t, "", ves[0].Field)
		assert.Equal(t, "some error", ves[0].Message)
		assert.Equal(t, genericErr, ves[0].Err)
	})

	t.Run("converts deeply wrapped ValidationError to ValidationErrors", func(t *testing.T) {
		ve := NewValidationError("field", "message")
		wrapped1 := fmt.Errorf("layer1: %w", ve)
		wrapped2 := fmt.Errorf("layer2: %w", wrapped1)
		wrapped3 := fmt.Errorf("layer3: %w", wrapped2)

		result := ConvertValidatorError(wrapped3)

		// Should be converted to ValidationErrors for consistency
		ves, ok := result.(ValidationErrors)
		require.True(t, ok, "expected ValidationErrors")
		require.Len(t, ves, 1)
		// The wrapped error message becomes the message
		assert.Contains(t, ves[0].Message, "layer3")
		assert.Contains(t, ves[0].Message, "layer2")
		assert.Contains(t, ves[0].Message, "layer1")
	})

	t.Run("handles deeply wrapped ValidationErrors", func(t *testing.T) {
		ves := ValidationErrors{
			NewValidationError("field1", "message1"),
			NewValidationError("field2", "message2"),
		}
		wrapped1 := fmt.Errorf("layer1: %w", ves)
		wrapped2 := fmt.Errorf("layer2: %w", wrapped1)

		result := ConvertValidatorError(wrapped2)

		assert.Equal(t, wrapped2, result)

		var unwrapped ValidationErrors
		assert.True(t, errors.As(result, &unwrapped))
		assert.Len(t, unwrapped, 2)
	})
}

func TestValidatePattern(t *testing.T) {
	t.Run("matching value passes", func(t *testing.T) {
		assert.NoError(t, ValidatePattern("ABC", `^[A-Z]{3}$`))
	})

	t.Run("non-matching value fails", func(t *testing.T) {
		err := ValidatePattern("abc", `^[A-Z]{3}$`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must match pattern")
		assert.Contains(t, err.Error(), `^[A-Z]{3}$`)
	})

	t.Run("empty value is skipped", func(t *testing.T) {
		// An empty value is treated as absent; a required constraint is
		// responsible for rejecting it.
		assert.NoError(t, ValidatePattern("", `^[A-Z]{3}$`))
	})

	t.Run("returned error is a ValidationError", func(t *testing.T) {
		err := ValidatePattern("abc", `^[A-Z]{3}$`)
		var ve ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Empty(t, ve.Field, "field should be supplied by the caller via Append")
	})

	t.Run("field is applied by Append", func(t *testing.T) {
		var verrs ValidationErrors
		verrs = verrs.Append("Code", ValidatePattern("abc", `^[A-Z]{3}$`))
		require.Len(t, verrs, 1)
		assert.Equal(t, "Code", verrs[0].Field)
	})

	t.Run("uncompilable pattern reports an error", func(t *testing.T) {
		// Lookahead is valid ECMA-262 but unsupported by Go's RE2 engine.
		err := ValidatePattern("anything", `(?=x)`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unusable pattern")
	})

	t.Run("dereferences pointers", func(t *testing.T) {
		s := "abc"
		assert.NoError(t, ValidatePattern(&s, `^[a-z]+$`))
		bad := "ABC"
		assert.Error(t, ValidatePattern(&bad, `^[a-z]+$`))
	})

	t.Run("nil pointer is skipped", func(t *testing.T) {
		var s *string
		assert.NoError(t, ValidatePattern(s, `^[a-z]+$`))
	})

	t.Run("named string types are matched", func(t *testing.T) {
		type code string
		assert.NoError(t, ValidatePattern(code("abc"), `^[a-z]+$`))
		assert.Error(t, ValidatePattern(code("ABC"), `^[a-z]+$`))
		c := code("abc")
		assert.NoError(t, ValidatePattern(&c, `^[a-z]+$`))
	})

	t.Run("non-string values are skipped", func(t *testing.T) {
		// A regex can't apply to a slice/struct/number, so these pass rather than error.
		assert.NoError(t, ValidatePattern([]string{"x"}, `^[a-z]+$`))
		assert.NoError(t, ValidatePattern(42, `^[a-z]+$`))
		assert.NoError(t, ValidatePattern(struct{ X int }{}, `^[a-z]+$`))
	})

	t.Run("compiled pattern is cached", func(t *testing.T) {
		const pattern = `^cache-[0-9]+$`
		require.NoError(t, ValidatePattern("cache-1", pattern))

		// Second call hits the cache and behaves identically.
		require.NoError(t, ValidatePattern("cache-2", pattern))

		_, ok := patternCache.Load(pattern)
		assert.True(t, ok, "pattern should be cached after first use")
	})
}
