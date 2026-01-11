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
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// ErrValidationEmail is the sentinel error returned when an email fails validation
var (
	ErrValidationEmail         = errors.New("email: failed to pass regex validation")
	ErrFailedToUnmarshalAsAOrB = errors.New("failed to unmarshal as either A or B")
	ErrMustBeMap               = errors.New("value must be map[string]any")
)

type ClientAPIErrorOption func(*ClientAPIError)

// ClientAPIError represents type for client API errors.
type ClientAPIError struct {
	err        error
	statusCode int
}

// Error implements the error interface.
func (e *ClientAPIError) Error() string {
	if e.err == nil {
		return "client api error"
	}
	return e.err.Error()
}

func (e *ClientAPIError) StatusCode() int {
	return e.statusCode
}

// Unwrap returns the underlying error.
func (e *ClientAPIError) Unwrap() error {
	return e.err
}

// NewClientAPIError creates a new ClientAPIError from the given error.
func NewClientAPIError(err error, opts ...ClientAPIErrorOption) error {
	e := &ClientAPIError{err: err}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func WithStatusCode(code int) ClientAPIErrorOption {
	return func(e *ClientAPIError) {
		e.statusCode = code
	}
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`

	// underlying error, not serialized
	Err error `json:"-"`
}

func (e ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s %s", e.Field, e.Message)
	}
	return e.Message
}

// Unwrap returns the underlying error for error wrapping support
func (e ValidationError) Unwrap() error {
	return e.Err
}

func NewValidationError(field, message string) ValidationError {
	return ValidationError{Field: field, Message: message}
}

// NewValidationErrorFromError creates ValidationErrors from a single error with a field prefix.
// This is a convenience wrapper around NewValidationErrorsFromErrors for a single error.
func NewValidationErrorFromError(field string, err error) error {
	result := NewValidationErrorsFromErrors(field, []error{err})
	if len(result) == 0 {
		return nil
	}
	return result
}

type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	var messages []string
	for _, e := range ve {
		if e.Field != "" {
			messages = append(messages, fmt.Sprintf("%s %s", e.Field, e.Message))
		} else {
			messages = append(messages, e.Message)
		}
	}
	return strings.Join(messages, "\n")
}

// Add adds a single ValidationError to the collection.
func (ve ValidationErrors) Add(field, message string) ValidationErrors {
	return append(ve, ValidationError{Field: field, Message: message})
}

// Append adds validation errors from the given error to the collection.
// It handles ValidationError, ValidationErrors, and other error types.
func (ve ValidationErrors) Append(field string, err error) ValidationErrors {
	if err == nil {
		return ve
	}

	newErrors := NewValidationErrorsFromErrors(field, []error{err})
	return append(ve, newErrors...)
}

func (ve ValidationErrors) Unwrap() []error {
	errs := make([]error, len(ve))
	for i, e := range ve {
		errs[i] = e
	}
	return errs
}

// NewValidationErrorsFromError creates a new ValidationErrors from a single error.
func NewValidationErrorsFromError(err error) ValidationErrors {
	return NewValidationErrorsFromErrors("", []error{err})
}

// NewValidationErrorsFromString creates a new ValidationErrors from a field and message.
func NewValidationErrorsFromString(field, message string) ValidationErrors {
	return ValidationErrors{{Field: field, Message: message}}
}

// NewValidationErrorsFromErrors creates a new ValidationErrors from a list of errors.
// If prefix is provided, it will be prepended to each field name with a dot.
func NewValidationErrorsFromErrors(prefix string, errs []error) ValidationErrors {
	var result ValidationErrors
	var validationErrors validator.ValidationErrors
	if prefix != "" {
		prefix = fmt.Sprintf("%s.", prefix)
	}

	for _, err := range errs {
		// Handle our ValidationErrors (plural) first - use direct type check to avoid unwrapping
		// Check the error directly, not through errors.As which would unwrap nested errors
		if ves, ok := err.(ValidationErrors); ok {
			for _, ve := range ves {
				// Create a copy to avoid modifying the original
				// Preserve the ValidationError itself as the underlying error to maintain the error chain
				newVe := ValidationError{
					Field:   ve.Field,
					Message: ve.Message,
					Err:     ve, // Preserve the ValidationError to maintain the error chain
				}
				if prefix != "" && newVe.Field != "" {
					prefixWithoutDot := strings.TrimSuffix(prefix, ".")
					// Avoid double-prefixing: if field already starts with prefix, don't add it again
					if !strings.HasPrefix(newVe.Field, prefixWithoutDot+".") && newVe.Field != prefixWithoutDot {
						newVe.Field = prefixWithoutDot + "." + newVe.Field
					}
				} else if prefix != "" {
					newVe.Field = strings.TrimSuffix(prefix, ".")
				}
				result = append(result, newVe)
			}
			continue
		}

		// Handle validator.ValidationErrors from go-playground/validator
		// Use errors.As here because validator.ValidationErrors might be wrapped
		if errors.As(err, &validationErrors) {
			for _, ve := range validationErrors {
				field := prefix + ve.Field()
				// If ve.Field() is empty (from validator.Var()), trim the trailing dot
				if ve.Field() == "" && prefix != "" {
					field = strings.TrimSuffix(prefix, ".")
				}
				result = append(result, ValidationError{
					Field:   field,
					Message: convertFieldErrorMessage(ve),
					Err:     err,
				})
			}
			continue
		}

		// Handle single ValidationError - use direct type check to avoid unwrapping
		if ve, ok := err.(ValidationError); ok {
			// Create a copy to avoid modifying the original
			newVe := ValidationError{
				Field:   ve.Field,
				Message: ve.Message,
				Err:     ve.Err,
			}
			if prefix != "" && newVe.Field != "" {
				prefixWithoutDot := strings.TrimSuffix(prefix, ".")
				// Avoid double-prefixing: if field already starts with prefix, don't add it again
				if !strings.HasPrefix(newVe.Field, prefixWithoutDot+".") && newVe.Field != prefixWithoutDot {
					newVe.Field = prefixWithoutDot + "." + newVe.Field
				}
			} else if prefix != "" {
				newVe.Field = strings.TrimSuffix(prefix, ".")
			}
			result = append(result, newVe)
			continue
		}

		// Handle generic errors - wrap them in a ValidationError
		field := strings.TrimSuffix(prefix, ".")
		result = append(result, ValidationError{
			Field:   field,
			Message: err.Error(),
			Err:     err,
		})
	}

	return result
}

func convertFieldErrorMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email"
	case "gt":
		return fmt.Sprintf("must be greater than %s", fe.Param())
	case "gte":
		return fmt.Sprintf("must be greater than or equal to %s", fe.Param())
	case "lt":
		return fmt.Sprintf("must be less than %s", fe.Param())
	case "lte":
		return fmt.Sprintf("must be less than or equal to %s", fe.Param())
	case "min":
		return fmt.Sprintf("length must be greater than or equal to %s", fe.Param())
	case "max":
		return fmt.Sprintf("length must be less than or equal to %s", fe.Param())
	default:
		return fmt.Sprintf("is not valid (%s)", fe.Tag())
	}
}
