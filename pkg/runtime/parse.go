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
	"strconv"
	"time"

	"github.com/google/uuid"
)

// ParseString parses a string into the target type T.
// Supports all Go primitive types: int, int8, int16, int32, int64,
// uint, uint8, uint16, uint32, uint64, float32, float64, bool, string,
// as well as special types like uuid.UUID and time.Time when the appropriate
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

	// Fall back to type switch for primitives
	switch p := any(&result).(type) {
	case *int:
		v, err := strconv.Atoi(s)
		*p = v
		return result, err
	case *int8:
		v, err := strconv.ParseInt(s, 10, 8)
		*p = int8(v)
		return result, err
	case *int16:
		v, err := strconv.ParseInt(s, 10, 16)
		*p = int16(v)
		return result, err
	case *int32:
		v, err := strconv.ParseInt(s, 10, 32)
		*p = int32(v)
		return result, err
	case *int64:
		v, err := strconv.ParseInt(s, 10, 64)
		*p = v
		return result, err
	case *uint:
		v, err := strconv.ParseUint(s, 10, 0)
		*p = uint(v)
		return result, err
	case *uint8:
		v, err := strconv.ParseUint(s, 10, 8)
		*p = uint8(v)
		return result, err
	case *uint16:
		v, err := strconv.ParseUint(s, 10, 16)
		*p = uint16(v)
		return result, err
	case *uint32:
		v, err := strconv.ParseUint(s, 10, 32)
		*p = uint32(v)
		return result, err
	case *uint64:
		v, err := strconv.ParseUint(s, 10, 64)
		*p = v
		return result, err
	case *float32:
		v, err := strconv.ParseFloat(s, 32)
		*p = float32(v)
		return result, err
	case *float64:
		v, err := strconv.ParseFloat(s, 64)
		*p = v
		return result, err
	case *bool:
		v, err := strconv.ParseBool(s)
		*p = v
		return result, err
	case *string:
		*p = s
		return result, nil
	}
	return result, nil
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
