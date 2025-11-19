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
}

func (e ValidationError) Error() string {
	field := e.Field
	if field != "" {
		field = fmt.Sprintf("%s ", field)
	}
	return fmt.Sprintf("%s%s", field, e.Message)
}

func NewValidationError(field, message string) ValidationError {
	return ValidationError{Field: field, Message: message}
}

type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	var messages []string
	for _, e := range ve {
		field := e.Field
		if field != "" {
			field = fmt.Sprintf("%s ", field)
		}
		messages = append(messages, fmt.Sprintf("%s%s", field, e.Message))
	}
	return strings.Join(messages, "\n")
}

// NewValidationErrorsFromError creates a new ValidationErrors from a single error.
func NewValidationErrorsFromError(err error) ValidationErrors {
	return NewValidationErrorsFromErrors("", []error{err})
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
		if errors.As(err, &validationErrors) {
			for _, ve := range validationErrors {
				result = append(result, ValidationError{
					Field:   prefix + ve.Field(),
					Message: convertFieldErrorMessage(ve),
				})
			}
			continue
		}

		var ve ValidationError
		if errors.As(err, &ve) {
			result = append(result, ve)
		}
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
