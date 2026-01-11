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

func TestNewValidationErrorsFromString(t *testing.T) {
	errs := NewValidationErrorsFromString("TransactionId", "Field is required")

	require.Len(t, errs, 1)
	assert.Equal(t, "TransactionId", errs[0].Field)
	assert.Equal(t, "Field is required", errs[0].Message)
	assert.Equal(t, "TransactionId Field is required", errs.Error())
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
		// Check field and message, but Err should be the original error
		assert.Equal(t, "Name", validationErrors[0].Field)
		assert.Equal(t, "is required", validationErrors[0].Message)
		assert.NotNil(t, validationErrors[0].Err)
		assert.Equal(t, "Email", validationErrors[1].Field)
		assert.Equal(t, "must be a valid email", validationErrors[1].Message)
		assert.NotNil(t, validationErrors[1].Err)
		assert.Equal(t, "Age", validationErrors[2].Field)
		assert.Equal(t, "must be greater than 18", validationErrors[2].Message)
		assert.NotNil(t, validationErrors[2].Err)
		assert.Equal(t, "Rate", validationErrors[3].Field)
		assert.Equal(t, "must be less than 100", validationErrors[3].Message)
		assert.NotNil(t, validationErrors[3].Err)
		assert.Equal(t, "Price", validationErrors[4].Field)
		assert.Equal(t, "must be less than or equal to 20", validationErrors[4].Message)
		assert.NotNil(t, validationErrors[4].Err)
		assert.Equal(t, "Qty", validationErrors[5].Field)
		assert.Equal(t, "must be greater than or equal to 10", validationErrors[5].Message)
		assert.NotNil(t, validationErrors[5].Err)
		assert.Equal(t, "Range", validationErrors[6].Field)
		assert.Equal(t, "must be less than 20", validationErrors[6].Message)
		assert.NotNil(t, validationErrors[6].Err)
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
	assert.Equal(t, "headers.Name", validationErrors[0].Field)
	assert.Equal(t, "is required", validationErrors[0].Message)
	assert.Equal(t, "headers.Email", validationErrors[1].Field)
	assert.Equal(t, "must be a valid email", validationErrors[1].Message)
	assert.Equal(t, "Foo", validationErrors[2].Field)
	assert.Equal(t, "is required", validationErrors[2].Message)
}

func TestNewValidationErrorFromError(t *testing.T) {
	t.Run("wraps error and preserves it", func(t *testing.T) {
		originalErr := errors.New("min length is 3")
		result := NewValidationErrorFromError("Name", originalErr)

		// Now always returns ValidationErrors for consistency
		validationErrs, ok := result.(ValidationErrors)
		require.True(t, ok, "expected ValidationErrors")
		require.Len(t, validationErrs, 1)

		assert.Equal(t, "Name", validationErrs[0].Field)
		assert.Equal(t, "min length is 3", validationErrs[0].Message)
		assert.Equal(t, "Name min length is 3", validationErrs.Error())

		assert.Equal(t, originalErr, validationErrs[0].Err)
		assert.True(t, errors.Is(validationErrs[0], originalErr))
	})

	t.Run("works with single validator error", func(t *testing.T) {
		validate := validator.New()
		err := validate.Var("ab", "min=3")
		require.Error(t, err)

		result := NewValidationErrorFromError("Name", err)

		// Now always returns ValidationErrors for consistency
		validationErrs, ok := result.(ValidationErrors)
		require.True(t, ok, "expected ValidationErrors")
		require.Len(t, validationErrs, 1)

		assert.Equal(t, "Name", validationErrs[0].Field)
		assert.Equal(t, "length must be greater than or equal to 3", validationErrs[0].Message)
		assert.Equal(t, "Name length must be greater than or equal to 3", validationErrs.Error())

		assert.NotNil(t, validationErrs[0].Err)
		assert.Equal(t, err, validationErrs[0].Unwrap())

		var validatorErr validator.ValidationErrors
		assert.True(t, errors.As(validationErrs[0], &validatorErr))
	})

	t.Run("validator.Var with required tag", func(t *testing.T) {
		validate := validator.New()
		err := validate.Var("", "required")
		require.Error(t, err)

		result := NewValidationErrorFromError("Payload", err)

		// Now always returns ValidationErrors for consistency
		validationErrs, ok := result.(ValidationErrors)
		require.True(t, ok, "expected ValidationErrors")
		require.Len(t, validationErrs, 1)

		// Should NOT have a dot after Payload
		assert.Equal(t, "Payload", validationErrs[0].Field)
		assert.Equal(t, "is required", validationErrs[0].Message)
		assert.Equal(t, "Payload is required", validationErrs.Error())
		assert.NotContains(t, validationErrs.Error(), "Payload.")
	})

	t.Run("multiple validator errors", func(t *testing.T) {
		validate := validator.New()

		// Validate a struct with multiple failing fields
		type TestStruct struct {
			Currency string `validate:"max=3"`
			Amount   int    `validate:"gte=0"`
		}

		ts := TestStruct{Currency: "TOOLONG", Amount: -5}
		err := validate.Struct(ts)
		require.Error(t, err)

		result := NewValidationErrorFromError("RefundPaymentRequest", err)

		// Should return ValidationErrors (plural) with all errors
		validationErrs, ok := result.(ValidationErrors)
		require.True(t, ok, "expected ValidationErrors for multiple errors")
		assert.Len(t, validationErrs, 2)

		// Check first error
		assert.Equal(t, "RefundPaymentRequest.Currency", validationErrs[0].Field)
		assert.Equal(t, "length must be less than or equal to 3", validationErrs[0].Message)

		// Check second error
		assert.Equal(t, "RefundPaymentRequest.Amount", validationErrs[1].Field)
		assert.Equal(t, "must be greater than or equal to 0", validationErrs[1].Message)

		// Check combined error message
		expected := "RefundPaymentRequest.Currency length must be less than or equal to 3\nRefundPaymentRequest.Amount must be greater than or equal to 0"
		assert.Equal(t, expected, validationErrs.Error())
	})

	t.Run("empty field", func(t *testing.T) {
		originalErr := errors.New("required")
		result := NewValidationErrorFromError("", originalErr)

		// Now always returns ValidationErrors for consistency
		validationErrs, ok := result.(ValidationErrors)
		require.True(t, ok, "expected ValidationErrors")
		require.Len(t, validationErrs, 1)

		assert.Equal(t, "", validationErrs[0].Field)
		assert.Equal(t, "required", validationErrs[0].Message)
		assert.Equal(t, "required", validationErrs.Error())
		assert.Equal(t, originalErr, validationErrs[0].Err)
	})

	t.Run("converts validator.FieldError messages", func(t *testing.T) {
		validate := validator.New()

		testCases := []struct {
			name            string
			value           any
			tag             string
			expectedMessage string
		}{
			{"required", "", "required", "is required"},
			{"email", "invalid", "email", "must be a valid email"},
			{"gt", 5, "gt=10", "must be greater than 10"},
			{"gte", 5, "gte=10", "must be greater than or equal to 10"},
			{"lt", 15, "lt=10", "must be less than 10"},
			{"lte", 15, "lte=10", "must be less than or equal to 10"},
			{"min", "ab", "min=3", "length must be greater than or equal to 3"},
			{"max", "abcdef", "max=3", "length must be less than or equal to 3"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := validate.Var(tc.value, tc.tag)
				require.Error(t, err)

				result := NewValidationErrorFromError("TestField", err)

				// Now always returns ValidationErrors for consistency
				validationErrs, ok := result.(ValidationErrors)
				require.True(t, ok, "expected ValidationErrors")
				require.Len(t, validationErrs, 1)

				assert.Equal(t, "TestField", validationErrs[0].Field)
				assert.Equal(t, tc.expectedMessage, validationErrs[0].Message)
				assert.Equal(t, "TestField "+tc.expectedMessage, validationErrs.Error())
				assert.Equal(t, err, validationErrs[0].Err)
			})
		}
	})

	t.Run("validates Amount field with gte=0", func(t *testing.T) {
		validate := validator.New()

		type Payment struct {
			Amount int64 `json:"amount" validate:"gte=0"`
		}

		t.Run("valid - positive amount", func(t *testing.T) {
			payment := Payment{Amount: 100}
			err := validate.Var(payment.Amount, "gte=0")
			assert.NoError(t, err)
		})

		t.Run("valid - zero amount", func(t *testing.T) {
			payment := Payment{Amount: 0}
			err := validate.Var(payment.Amount, "gte=0")
			assert.NoError(t, err)
		})

		t.Run("invalid - negative amount", func(t *testing.T) {
			payment := Payment{Amount: -10}
			err := validate.Var(payment.Amount, "gte=0")
			require.Error(t, err)

			result := NewValidationErrorFromError("Amount", err)

			// Now always returns ValidationErrors for consistency
			validationErrs, ok := result.(ValidationErrors)
			require.True(t, ok, "expected ValidationErrors")
			require.Len(t, validationErrs, 1)

			assert.Equal(t, "Amount", validationErrs[0].Field)
			assert.Equal(t, "must be greater than or equal to 0", validationErrs[0].Message)
			assert.Equal(t, "Amount must be greater than or equal to 0", validationErrs.Error())
			assert.Equal(t, err, validationErrs[0].Err)
		})
	})
}

func TestValidationError_Unwrap(t *testing.T) {
	t.Run("returns underlying error", func(t *testing.T) {
		originalErr := errors.New("test error")
		result := NewValidationErrorFromError("Field", originalErr)

		// Now always returns ValidationErrors for consistency
		validationErrs, ok := result.(ValidationErrors)
		require.True(t, ok, "expected ValidationErrors")
		require.Len(t, validationErrs, 1)

		unwrapped := validationErrs[0].Unwrap()
		assert.Equal(t, originalErr, unwrapped)
	})

	t.Run("returns nil when no underlying error", func(t *testing.T) {
		validationErr := NewValidationError("Field", "message")

		unwrapped := validationErr.Unwrap()
		assert.Nil(t, unwrapped)
	})

	t.Run("works with errors.Is", func(t *testing.T) {
		sentinelErr := errors.New("sentinel")
		validationErr := NewValidationErrorFromError("Field", sentinelErr)

		assert.True(t, errors.Is(validationErr, sentinelErr))
	})

	t.Run("works with errors.As", func(t *testing.T) {
		validate := validator.New()
		validatorErr := validate.Var("", "required")
		validationErr := NewValidationErrorFromError("Field", validatorErr)

		var ve validator.ValidationErrors
		assert.True(t, errors.As(validationErr, &ve))
	})
}

func TestNewValidationErrorsFromErrors_MultipleErrors(t *testing.T) {
	t.Run("handles multiple ValidationError instances", func(t *testing.T) {
		errs := []error{
			NewValidationError("foo", "is required"),
			NewValidationError("bar", "is nice to have"),
		}

		result := NewValidationErrorsFromErrors("", errs)

		assert.Len(t, result, 2)
		assert.Equal(t, "foo", result[0].Field)
		assert.Equal(t, "is required", result[0].Message)
		assert.Equal(t, "bar", result[1].Field)
		assert.Equal(t, "is nice to have", result[1].Message)

		// Test the combined error message
		expected := "foo is required\nbar is nice to have"
		assert.Equal(t, expected, result.Error())
	})

	t.Run("handles validator.Var errors with empty prefix", func(t *testing.T) {
		validate := validator.New()
		err := validate.Var("", "required")
		require.Error(t, err)

		result := NewValidationErrorsFromErrors("", []error{err})

		assert.Len(t, result, 1)
		// When both prefix and ve.Field() are empty, field should be empty
		assert.Equal(t, "", result[0].Field)
		assert.Equal(t, "is required", result[0].Message)
		// Error message should be just the message, no trailing dot
		assert.Equal(t, "is required", result[0].Error())
		assert.NotContains(t, result[0].Error(), ".")
	})

	t.Run("handles ValidationErrors (plural) type", func(t *testing.T) {
		// Create a ValidationErrors collection
		ves := ValidationErrors{
			NewValidationError("field1", "error1"),
			NewValidationError("field2", "error2"),
		}

		// Pass it to NewValidationErrorsFromErrors
		result := NewValidationErrorsFromErrors("", []error{ves})

		// Should preserve all errors
		assert.Len(t, result, 2)
		assert.Equal(t, "field1", result[0].Field)
		assert.Equal(t, "error1", result[0].Message)
		assert.Equal(t, "field2", result[1].Field)
		assert.Equal(t, "error2", result[1].Message)
	})

	t.Run("handles ValidationErrors with prefix", func(t *testing.T) {
		ves := ValidationErrors{
			NewValidationError("field1", "error1"),
			NewValidationError("field2", "error2"),
		}

		result := NewValidationErrorsFromErrors("Body", []error{ves})

		assert.Len(t, result, 2)
		assert.Equal(t, "Body.field1", result[0].Field)
		assert.Equal(t, "Body.field2", result[1].Field)
	})

	t.Run("NewValidationErrorFromError wraps ValidationErrors with prefix", func(t *testing.T) {
		// Create a ValidationErrors collection
		ves := ValidationErrors{
			NewValidationError("foo", "is required"),
			NewValidationError("bar", "is nice to have"),
		}

		// Pass it to NewValidationErrorFromError (singular)
		// When there are multiple errors, it returns ValidationErrors (plural)
		result := NewValidationErrorFromError("Body", ves)

		// Should return ValidationErrors with all errors prefixed
		resultErrs, ok := result.(ValidationErrors)
		require.True(t, ok, "expected ValidationErrors for multiple errors")
		assert.Len(t, resultErrs, 2)
		assert.Equal(t, "Body.foo", resultErrs[0].Field)
		assert.Equal(t, "is required", resultErrs[0].Message)
		assert.Equal(t, "Body.bar", resultErrs[1].Field)
		assert.Equal(t, "is nice to have", resultErrs[1].Message)
	})

	t.Run("wraps ValidationErrors from Validate() with prefix - simulates user code", func(t *testing.T) {
		validate := validator.New()

		// Simulate what request.Validate() returns
		type CapturePaymentRequest struct {
			Currency string `validate:"required,max=3,min=3"`
			Amount   int    `validate:"gte=0"`
		}

		req := CapturePaymentRequest{
			Currency: "TOOLONG", // Invalid: too long
			Amount:   -5,        // Invalid: negative
		}

		// This is what happens inside request.Validate()
		err := validate.Struct(req)
		require.Error(t, err)

		// Convert to our ValidationErrors (this is what Validate() returns)
		validationErrs := ConvertValidatorError(err)
		require.NotNil(t, validationErrs)

		// Now user wraps it with prefix (this is what user does)
		result := NewValidationErrorFromError("CapturePaymentRequest", validationErrs)

		// Should get ValidationErrors with both errors prefixed
		resultErrs, ok := result.(ValidationErrors)
		require.True(t, ok, "expected ValidationErrors")
		require.Len(t, resultErrs, 2, "should have 2 errors (Currency and Amount)")

		// Check we have both errors with correct prefixes
		assert.Equal(t, "CapturePaymentRequest.Currency", resultErrs[0].Field)
		assert.Contains(t, resultErrs[0].Message, "length must be")
		assert.Equal(t, "CapturePaymentRequest.Amount", resultErrs[1].Field)
		assert.Contains(t, resultErrs[1].Message, "must be greater than or equal to")

		// Print for debugging
		t.Logf("Result errors:\n%s", resultErrs.Error())
	})

	t.Run("avoids double-prefixing when field already has prefix", func(t *testing.T) {
		// Simulate: request.Validate() returns errors with fields already prefixed
		ves := ValidationErrors{
			NewValidationError("RefundPaymentRequest.Amount", "is required"),
			NewValidationError("RefundPaymentRequest.Currency", "is invalid"),
		}

		// User wraps it again with the same prefix (common mistake)
		result := NewValidationErrorsFromErrors("RefundPaymentRequest", []error{ves})

		// Should NOT double-prefix
		assert.Len(t, result, 2)
		assert.Equal(t, "RefundPaymentRequest.Amount", result[0].Field)
		assert.Equal(t, "RefundPaymentRequest.Currency", result[1].Field)
	})

	t.Run("avoids double-prefixing for single ValidationError", func(t *testing.T) {
		// Single error already has the prefix
		ve := NewValidationError("Foo.Bar", "is invalid")

		// Wrap it again with "Foo"
		result := NewValidationErrorsFromErrors("Foo", []error{ve})

		// Should NOT double-prefix
		assert.Len(t, result, 1)
		assert.Equal(t, "Foo.Bar", result[0].Field)
	})

	t.Run("still adds prefix when field doesn't have it", func(t *testing.T) {
		// Error without prefix
		ves := ValidationErrors{
			NewValidationError("Amount", "is required"),
			NewValidationError("Currency", "is invalid"),
		}

		// Add prefix
		result := NewValidationErrorsFromErrors("RefundPaymentRequest", []error{ves})

		// Should add prefix
		assert.Len(t, result, 2)
		assert.Equal(t, "RefundPaymentRequest.Amount", result[0].Field)
		assert.Equal(t, "RefundPaymentRequest.Currency", result[1].Field)
	})

	t.Run("use NewValidationErrorsFromErrors for multiple errors", func(t *testing.T) {
		// When you want to preserve all errors, use NewValidationErrorsFromErrors
		errs := []error{
			NewValidationError("foo", "is required"),
			NewValidationError("bar", "is nice to have"),
		}

		result := NewValidationErrorsFromErrors("Body", errs)

		// All errors are preserved with prefix
		assert.Len(t, result, 2)
		assert.Equal(t, "Body.foo", result[0].Field)
		assert.Equal(t, "is required", result[0].Message)
		assert.Equal(t, "Body.bar", result[1].Field)
		assert.Equal(t, "is nice to have", result[1].Message)

		// The combined error message
		expected := "Body.foo is required\nBody.bar is nice to have"
		assert.Equal(t, expected, result.Error())
	})
}
