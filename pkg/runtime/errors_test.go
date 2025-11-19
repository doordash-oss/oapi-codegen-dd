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
	"errors"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClientAPIError(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		err := NewClientAPIError(nil)
		assert.Equal(t, "client api error", err.Error())
	})

	t.Run("non-nil error", func(t *testing.T) {
		err := NewClientAPIError(ErrValidationEmail)
		assert.Equal(t, ErrValidationEmail.Error(), err.Error())
	})

	t.Run("non-nil error with status code", func(t *testing.T) {
		err := NewClientAPIError(ErrValidationEmail, WithStatusCode(400))
		assert.Equal(t, ErrValidationEmail.Error(), err.Error())

		var apiErr *ClientAPIError
		require.True(t, errors.As(err, &apiErr))
		assert.Equal(t, 400, apiErr.StatusCode())
	})
}

func TestNewValidationError(t *testing.T) {
	t.Run("empty field", func(t *testing.T) {
		err := NewValidationError("", "is required")
		assert.Equal(t, "is required", err.Error())
	})

	t.Run("non-empty field", func(t *testing.T) {
		err := NewValidationError("foo", "is required")
		assert.Equal(t, "foo is required", err.Error())
	})
}

func TestNewValidationErrorsFromError(t *testing.T) {
	validate := validator.New(validator.WithRequiredStructEnabled())
	type Foo struct {
		Name  string `validate:"required"`
		Email string `validate:"required,email"`
		Age   int    `validate:"gt=18"`
		Rate  int    `validate:"lt=100"`
		Price int    `validate:"lte=20"`
		Qty   int    `validate:"gte=10"`
		Range int    `validate:"gte=10,lt=20"`
	}

	t.Run("multiple errors", func(t *testing.T) {
		foo := Foo{
			Email: "email",
			Rate:  101,
			Price: 21,
			Qty:   9,
			Range: 20,
		}
		err := validate.Struct(foo)
		validationErrors := NewValidationErrorsFromError(err)

		assert.Len(t, validationErrors, 7)
		assert.Equal(t, ValidationError{"Name", "is required"}, validationErrors[0])
		assert.Equal(t, ValidationError{"Email", "must be a valid email"}, validationErrors[1])
		assert.Equal(t, ValidationError{"Age", "must be greater than 18"}, validationErrors[2])
		assert.Equal(t, ValidationError{"Rate", "must be less than 100"}, validationErrors[3])
		assert.Equal(t, ValidationError{"Price", "must be less than or equal to 20"}, validationErrors[4])
		assert.Equal(t, ValidationError{"Qty", "must be greater than or equal to 10"}, validationErrors[5])
		assert.Equal(t, ValidationError{"Range", "must be less than 20"}, validationErrors[6])
	})
}

func TestNewValidationErrorsFromErrors(t *testing.T) {
	validate := validator.New(validator.WithRequiredStructEnabled())
	type Foo struct {
		Name  string `validate:"required"`
		Email string `validate:"required,email"`
	}

	foo := Foo{
		Name:  "",
		Email: "email",
	}
	err := validate.Struct(foo)
	validationErrors := NewValidationErrorsFromErrors("headers", []error{err})
	validationErrors = append(validationErrors, NewValidationError("Foo", "is required"))

	assert.Len(t, validationErrors, 3)
	assert.Equal(t, ValidationError{"headers.Name", "is required"}, validationErrors[0])
	assert.Equal(t, ValidationError{"headers.Email", "must be a valid email"}, validationErrors[1])
	assert.Equal(t, ValidationError{"Foo", "is required"}, validationErrors[2])
}
