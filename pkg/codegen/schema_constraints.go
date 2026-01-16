// Copyright 2025 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package codegen

import (
	"fmt"
	"slices"
	"sort"

	"github.com/pb33f/libopenapi/datamodel/high/base"
)

type ConstraintsContext struct {
	name         string
	hasNilType   bool
	required     bool
	specLocation SpecLocation
}

type Constraints struct {
	Required       *bool
	Nullable       *bool
	ReadOnly       *bool
	WriteOnly      *bool
	MinLength      *int64
	MaxLength      *int64
	Pattern        *string
	Min            *float64
	Max            *float64
	MinItems       *int64
	MaxItems       *int64
	MinProperties  *int64
	MaxProperties  *int64
	ValidationTags []string
}

func (c Constraints) IsEqual(other Constraints) bool {
	return ptrEqual(c.Required, other.Required) &&
		ptrEqual(c.Nullable, other.Nullable) &&
		ptrEqual(c.ReadOnly, other.ReadOnly) &&
		ptrEqual(c.WriteOnly, other.WriteOnly) &&
		ptrEqual(c.MinLength, other.MinLength) &&
		ptrEqual(c.MaxLength, other.MaxLength) &&
		ptrEqual(c.Pattern, other.Pattern) &&
		ptrEqual(c.Min, other.Min) &&
		ptrEqual(c.Max, other.Max) &&
		ptrEqual(c.MinItems, other.MinItems) &&
		ptrEqual(c.MaxItems, other.MaxItems) &&
		ptrEqual(c.MinProperties, other.MinProperties) &&
		ptrEqual(c.MaxProperties, other.MaxProperties) &&
		slices.Equal(c.ValidationTags, other.ValidationTags)
}

// Count returns the number of validation constraints.
// This is useful for determining which schema is "stricter" when deduplicating union elements.
func (c Constraints) Count() int {
	count := 0

	// String constraints
	if c.MinLength != nil {
		count++
	}
	if c.MaxLength != nil {
		count++
	}
	if c.Pattern != nil {
		count++
	}

	// Numeric constraints
	if c.Min != nil {
		count++
	}
	if c.Max != nil {
		count++
	}

	// Array constraints
	if c.MinItems != nil {
		count++
	}
	if c.MaxItems != nil {
		count++
	}

	// Object constraints
	if c.MinProperties != nil {
		count++
	}
	if c.MaxProperties != nil {
		count++
	}

	// ValidationTags includes additional constraints like enum, format, etc.
	// Each tag represents a constraint
	count += len(c.ValidationTags)

	return count
}

func newConstraints(schema *base.Schema, opts ConstraintsContext) Constraints {
	if schema == nil {
		return Constraints{}
	}

	isInt := slices.Contains(schema.Type, "integer")
	isFloat := slices.Contains(schema.Type, "number")
	isBoolean := slices.Contains(schema.Type, "boolean")
	isString := slices.Contains(schema.Type, "string")

	// Check if the string format converts to a non-string Go type.
	// These formats do not support minLength/maxLength validation tags because
	// the Go type is not a string (e.g., time.Time, uuid.UUID).
	hasNonStringFormat := isString && (schema.Format == "date-time" || schema.Format == "date" || schema.Format == "uuid")
	isArray := slices.Contains(schema.Type, "array")
	isObject := schema.Type == nil || slices.Contains(schema.Type, "object")
	var validationTags []string

	name := opts.name
	hasNilType := opts.hasNilType

	required := opts.required
	if !required && name != "" {
		required = slices.Contains(schema.Required, name)
	}

	// ReadOnly fields should not have struct-level required validation.
	// They are only present in responses, not in requests, so requiring them
	// at the struct level would fail when validating request bodies.
	// Component schemas are shared between requests and responses, so we can't
	// rely on specLocation to determine the context.
	if required && schema.ReadOnly != nil && *schema.ReadOnly {
		required = false
	}

	// WriteOnly fields should not have struct-level required validation.
	// They are only present in requests, not in responses, so requiring them
	// at the struct level would fail when validating response bodies.
	// Component schemas are shared between requests and responses, so we can't
	// rely on specLocation to determine the context.
	if required && schema.WriteOnly != nil && *schema.WriteOnly {
		required = false
	}

	nullable := !required || hasNilType || deref(schema.Nullable)

	if required && isBoolean {
		// otherwise validation will always fail with `false` value.
		required = false
		nullable = hasNilType
	}

	// Don't add "required" validation tag for object types (structs)
	// Objects should be validated by calling their .Validate() method, not by validator.Var()
	if required && isObject {
		required = false
		// Still mark as nullable if it's not required, so we get omitempty
		if hasNilType {
			nullable = true
		}
	}

	if required && nullable {
		nullable = true
	}
	if required {
		validationTags = append(validationTags, "required")
	} else if nullable {
		validationTags = append(validationTags, "omitempty")
	}

	var readOnly *bool
	if schema.ReadOnly != nil {
		readOnly = schema.ReadOnly
	}

	var writeOnly *bool
	if schema.WriteOnly != nil {
		writeOnly = schema.WriteOnly
	}

	var minValue *float64
	// Only store minimum for numeric types (integer/number)
	// For strings, minimum is invalid per OpenAPI spec - ignore it completely
	if schema.Minimum != nil && (isInt || isFloat) {
		minTag := "gte"
		val := *schema.Minimum
		if schema.ExclusiveMinimum != nil && ((schema.ExclusiveMinimum.IsA() && schema.ExclusiveMinimum.A) || schema.ExclusiveMinimum.IsB()) {
			minTag = "gt"
			if schema.ExclusiveMinimum.IsB() {
				val = schema.ExclusiveMinimum.B
			}
		}

		minValue = &val
		tag := fmt.Sprintf("%s=%g", minTag, val)
		if isInt {
			tag = fmt.Sprintf("%s=%d", minTag, int64(val))
		}
		validationTags = append(validationTags, tag)
	}

	var maxValue *float64
	// Only store maximum for numeric types (integer/number)
	// For strings, maximum is invalid per OpenAPI spec - ignore it completely
	if schema.Maximum != nil && (isInt || isFloat) {
		maxTag := "lte"
		val := *schema.Maximum
		if schema.ExclusiveMaximum != nil && ((schema.ExclusiveMaximum.IsA() && schema.ExclusiveMaximum.A) || schema.ExclusiveMaximum.IsB()) {
			maxTag = "lt"
			if schema.ExclusiveMaximum.IsB() {
				val = schema.ExclusiveMaximum.B
			}
		}

		maxValue = &val
		tag := fmt.Sprintf("%s=%g", maxTag, val)
		if isInt {
			tag = fmt.Sprintf("%s=%d", maxTag, int64(val))
		}
		validationTags = append(validationTags, tag)
	}

	var minLength *int64
	// Only store minLength for strings and arrays
	// For integers/numbers/booleans, minLength is invalid per OpenAPI spec - ignore it completely
	if schema.MinLength != nil && (isString || isArray) && !hasNonStringFormat {
		minLength = schema.MinLength
		validationTags = append(validationTags, fmt.Sprintf("min=%d", *minLength))
	}

	var maxLength *int64
	// Only store maxLength for strings and arrays
	// For integers/numbers/booleans, maxLength is invalid per OpenAPI spec - ignore it completely
	if schema.MaxLength != nil && (isString || isArray) && !hasNonStringFormat {
		maxLength = schema.MaxLength
		validationTags = append(validationTags, fmt.Sprintf("max=%d", *maxLength))
	}

	var pattern *string
	if schema.Pattern != "" {
		pattern = &schema.Pattern
	}

	var minItems *int64
	if schema.MinItems != nil {
		minItems = schema.MinItems
	}

	var maxItems *int64
	if schema.MaxItems != nil {
		maxItems = schema.MaxItems
	}

	var minProperties *int64
	if schema.MinProperties != nil {
		minProperties = schema.MinProperties
	}

	var maxProperties *int64
	if schema.MaxProperties != nil {
		maxProperties = schema.MaxProperties
	}

	if len(validationTags) == 1 && validationTags[0] == "omitempty" {
		validationTags = nil
	}

	// place required, omitempty first in the list, then sort the rest
	sort.Slice(validationTags, func(i, j int) bool {
		a, b := validationTags[i], validationTags[j]

		// Define priority order
		priority := func(tag string) int {
			switch tag {
			case "required":
				return 0
			case "omitempty":
				return 1
			default:
				return 2
			}
		}

		pa, pb := priority(a), priority(b)
		if pa != pb {
			return pa < pb
		}
		return a < b
	})

	var requiredPtr *bool
	if required {
		requiredPtr = ptr(true)
	}

	var nullablePtr *bool
	if nullable {
		nullablePtr = ptr(true)
	}

	return Constraints{
		Nullable:       nullablePtr,
		Required:       requiredPtr,
		ReadOnly:       readOnly,
		WriteOnly:      writeOnly,
		Min:            minValue,
		Max:            maxValue,
		MinLength:      minLength,
		MaxLength:      maxLength,
		Pattern:        pattern,
		MinItems:       minItems,
		MaxItems:       maxItems,
		MinProperties:  minProperties,
		MaxProperties:  maxProperties,
		ValidationTags: validationTags,
	}
}
