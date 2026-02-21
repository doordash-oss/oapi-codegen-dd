// Copyright 2026 DoorDash, Inc.
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
	"strconv"
	"time"

	"github.com/google/uuid"
)

// ParseString parses a string into the target type T.
// Supports all Go primitive types: int, int8, int16, int32, int64,
// uint, uint8, uint16, uint32, uint64, float32, float64, bool, string,
// as well as type aliases of these primitives (e.g., type MyInt int),
// and special types like uuid.UUID and time.Time when the appropriate
// format hint is provided.
//
// The optional format parameter is the OpenAPI format (e.g., "uuid", "date-time", "date").
func ParseString[T any](s string, format ...string) (T, error) {
	var result T

	// Check format hint first for special types
	if len(format) > 0 {
		switch format[0] {
		case "uuid":
			if p, ok := any(&result).(*uuid.UUID); ok {
				v, err := uuid.Parse(s)
				if err != nil {
					return result, err
				}
				*p = v
				return result, nil
			}
		case "date-time":
			if p, ok := any(&result).(*time.Time); ok {
				v, err := time.Parse(time.RFC3339, s)
				if err != nil {
					return result, err
				}
				*p = v
				return result, nil
			}
		case "date":
			if p, ok := any(&result).(*Date); ok {
				v, err := time.Parse("2006-01-02", s)
				if err != nil {
					return result, err
				}
				*p = Date{Time: v}
				return result, nil
			}
		}
	}

	// Use reflection to handle both primitive types and their aliases
	// (e.g., type MyString string, type StatusCode int)
	// Type switches don't match aliased types, so we use reflect.Kind
	rv := reflect.ValueOf(&result).Elem()

	switch rv.Kind() {
	case reflect.String:
		rv.SetString(s)
		return result, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := strconv.ParseInt(s, 10, rv.Type().Bits())
		if err != nil {
			return result, err
		}
		rv.SetInt(v)
		return result, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v, err := strconv.ParseUint(s, 10, rv.Type().Bits())
		if err != nil {
			return result, err
		}
		rv.SetUint(v)
		return result, nil
	case reflect.Float32, reflect.Float64:
		v, err := strconv.ParseFloat(s, rv.Type().Bits())
		if err != nil {
			return result, err
		}
		rv.SetFloat(v)
		return result, nil
	case reflect.Bool:
		v, err := strconv.ParseBool(s)
		if err != nil {
			return result, err
		}
		rv.SetBool(v)
		return result, nil
	default:
		// Unsupported type - return zero value
		// This handles struct types, slices, maps, etc. that can't be parsed from a string
		return result, nil
	}
}

// ParseStringSlice parses a slice of strings into a slice of the target type T.
// Returns an error if any value fails to parse.
// The optional format parameter is the OpenAPI format (e.g., "uuid", "date-time", "date").
func ParseStringSlice[T any](values []string, format ...string) ([]T, error) {
	result := make([]T, len(values))
	for i, v := range values {
		parsed, err := ParseString[T](v, format...)
		if err != nil {
			return nil, err
		}
		result[i] = parsed
	}
	return result, nil
}
