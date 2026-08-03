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
	"reflect"
	"regexp"
	"sync"

	"github.com/go-playground/validator/v10"
)

// Validator is an interface for types that can validate themselves.
type Validator interface {
	Validate() error
}

// patternCache memoizes compiled regexps by source pattern; safe for concurrent use.
var patternCache sync.Map // map[string]*regexp.Regexp

// ValidatePattern reports whether value matches the OpenAPI `pattern` regex, for generated
// Validate() methods. value may be a string, a named string type, or a pointer to either;
// nil, non-string, and empty values are skipped (pair with `required` to reject empties).
// An uncompilable pattern yields a ValidationError.
func ValidatePattern(value any, pattern string) error {
	s, ok := patternInput(value)
	if !ok || s == "" {
		return nil
	}

	re, err := compilePattern(pattern)
	if err != nil {
		return NewValidationError("", fmt.Sprintf("has an unusable pattern '%s': %s", pattern, err))
	}

	if !re.MatchString(s) {
		return NewValidationError("", fmt.Sprintf("must match pattern '%s'", pattern))
	}
	return nil
}

// RegisterCustomTypeFunc registers a custom type function with the validator
// to extract values from types that have a Value() interface{} method.
// This is useful for union types (like Either) where only the active variant
// should be validated instead of all possible variants.
//
// The function looks for any struct with a Value() method matching the signature:
//
//	func Value() interface{}
//
// When found, it calls Value() and returns the result for validation instead of
// validating the struct's fields directly.
//
// Note: The struct fields should also have `validate:"-"` tags to prevent the
// validator from validating them directly. This function only affects validation
// when using validator.Var() on the struct itself.
func RegisterCustomTypeFunc(v *validator.Validate) {
	v.RegisterCustomTypeFunc(func(field reflect.Value) interface{} {
		// Check if this field is a struct with a Value() method
		if field.Kind() == reflect.Struct {
			valueMethod := field.MethodByName("Value")
			// Verify the method signature matches: func() interface{}
			// NumIn() == 0: method takes no parameters (no input arguments)
			// NumOut() == 1: method returns exactly one value (the active variant)
			if valueMethod.IsValid() && valueMethod.Type().NumIn() == 0 && valueMethod.Type().NumOut() == 1 {
				results := valueMethod.Call(nil)
				if len(results) > 0 && results[0].IsValid() && !results[0].IsNil() {
					return results[0].Interface()
				}
			}
		}
		return nil
	})
}

// ConvertValidatorError converts a validator.ValidationErrors to our ValidationErrors type.
// This provides a consistent error format across all validation errors.
func ConvertValidatorError(err error) error {
	if err == nil {
		return nil
	}

	// Check if it's already our ValidationErrors type using errors.As to handle wrapped errors
	var ves ValidationErrors
	if errors.As(err, &ves) {
		return err
	}

	// Use the existing NewValidationErrorsFromError which handles validator errors properly
	return NewValidationErrorsFromError(err)
}

// compilePattern returns the compiled form of pattern, caching the result.
func compilePattern(pattern string) (*regexp.Regexp, error) {
	// patternCache holds any; assert with comma-ok so a future change caching a
	// non-*regexp.Regexp recompiles instead of panicking.
	if cached, ok := patternCache.Load(pattern); ok {
		if re, ok := cached.(*regexp.Regexp); ok {
			return re, nil
		}
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	actual, _ := patternCache.LoadOrStore(pattern, re)
	if cached, ok := actual.(*regexp.Regexp); ok {
		return cached, nil
	}
	return re, nil
}

// patternInput extracts the string to match from value, dereferencing one pointer
// level and accepting named string types (e.g. type Email string). It returns false
// when value is nil or not string-kinded, so a regex over a non-string (slice, struct,
// number, ...) is skipped rather than misapplied or causing a type error.
func patternInput(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case *string:
		if v == nil {
			return "", false
		}
		return *v, true
	}

	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return "", false
		}
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.String {
		return rv.String(), true
	}
	return "", false
}
