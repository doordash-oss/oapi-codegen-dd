// Copyright 2025 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package slicespointer

import (
	"errors"
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPayments_Validate_PreservesOriginalError(t *testing.T) {
	t.Run("validation error preserves underlying validator error", func(t *testing.T) {
		// Create a Payments slice with an item that's too short (min=3)
		payments := Payments{"ab"}

		err := payments.Validate()
		require.Error(t, err)

		// Check that it's a ValidationError
		var validationErr runtime.ValidationError
		require.True(t, errors.As(err, &validationErr))

		// Check the field name
		assert.Equal(t, "[0]", validationErr.Field)

		// Check that we can unwrap to get the original validator error
		unwrapped := validationErr.Unwrap()
		require.NotNil(t, unwrapped)

		// Check that we can use errors.As to get the validator.ValidationErrors
		var validatorErrs validator.ValidationErrors
		assert.True(t, errors.As(err, &validatorErrs))
		assert.Len(t, validatorErrs, 1)
	})

	t.Run("validation error message shows actual error", func(t *testing.T) {
		payments := Payments{"ab"}

		err := payments.Validate()
		require.Error(t, err)

		// The error message should contain the actual validation error, not "validation failed"
		assert.Contains(t, err.Error(), "[0]")
		// Should not contain generic wrapper text
		assert.NotContains(t, err.Error(), "validation failed")
		assert.NotContains(t, err.Error(), "invalid item")
	})

	t.Run("min items constraint error", func(t *testing.T) {
		// Empty slice violates minItems=1
		payments := Payments{}

		err := payments.Validate()
		require.Error(t, err)

		var validationErr runtime.ValidationError
		require.True(t, errors.As(err, &validationErr))

		// This error is created directly, not from validator
		assert.Equal(t, "Array must have at least 1 items, got 0", err.Error())
	})
}

func TestUser_Validate_PreservesOriginalError(t *testing.T) {
	t.Run("nested validation error preserves chain", func(t *testing.T) {
		user := User{
			Payments: Payments{"ab"}, // Too short
		}

		err := user.Validate()
		require.Error(t, err)

		var validationErrs runtime.ValidationErrors
		require.True(t, errors.As(err, &validationErrs))
		require.Len(t, validationErrs, 1)

		// The field path should include the full path: Payments.[0]
		assert.Equal(t, "Payments.[0]", validationErrs[0].Field)

		// The underlying error should be preserved
		unwrapped := validationErrs[0].Unwrap()
		require.NotNil(t, unwrapped)

		// The unwrapped error should be a ValidationError with field "[0]"
		var nestedErr runtime.ValidationError
		assert.True(t, errors.As(unwrapped, &nestedErr))
		assert.Equal(t, "[0]", nestedErr.Field)

		var validatorErrs validator.ValidationErrors
		assert.True(t, errors.As(err, &validatorErrs))
	})
}
