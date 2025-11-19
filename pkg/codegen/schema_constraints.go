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
	name       string
	hasNilType bool
	required   bool
}

type Constraints struct {
	Required       bool
	Nullable       bool
	ReadOnly       bool
	WriteOnly      bool
	MinLength      int64
	MaxLength      int64
	Min            float64
	Max            float64
	MinItems       int
	ValidationTags []string
}

func (c Constraints) IsEqual(other Constraints) bool {
	return c.Required == other.Required &&
		c.Nullable == other.Nullable &&
		c.ReadOnly == other.ReadOnly &&
		c.WriteOnly == other.WriteOnly &&
		c.MinLength == other.MinLength &&
		c.MaxLength == other.MaxLength &&
		c.Min == other.Min &&
		c.Max == other.Max &&
		c.MinItems == other.MinItems &&
		slices.Equal(c.ValidationTags, other.ValidationTags)
}

func newConstraints(schema *base.Schema, opts ConstraintsContext) Constraints {
	if schema == nil {
		return Constraints{}
	}

	isInt := slices.Contains(schema.Type, "integer")
	isFloat := slices.Contains(schema.Type, "number")
	isBoolean := slices.Contains(schema.Type, "boolean")
	var validationTags []string

	name := opts.name
	hasNilType := opts.hasNilType

	required := opts.required
	if !required && name != "" {
		required = slices.Contains(schema.Required, name)
	}

	nullable := false
	if !required || hasNilType {
		nullable = true
	} else if schema.Nullable != nil {
		nullable = *schema.Nullable
	}

	if required && isBoolean {
		// otherwise validation will always fail with `false` value.
		required = false
		nullable = hasNilType
	}

	if required && nullable {
		nullable = true
	}
	if required {
		validationTags = append(validationTags, "required")
	} else if nullable {
		validationTags = append(validationTags, "omitempty")
	}

	readOnly := false
	if schema.ReadOnly != nil {
		readOnly = *schema.ReadOnly
	}

	writeOnly := false
	if schema.WriteOnly != nil {
		writeOnly = *schema.WriteOnly
	}

	minValue := float64(0)
	if schema.Minimum != nil {
		minTag := "gte"
		minValue = *schema.Minimum
		if schema.ExclusiveMinimum != nil {
			if schema.ExclusiveMinimum.IsA() && schema.ExclusiveMinimum.A {
				minTag = "gt"
			} else if schema.ExclusiveMinimum.IsB() {
				minTag = "gt"
				minValue = schema.ExclusiveMinimum.B
			}
		}
		if isInt {
			validationTags = append(validationTags, fmt.Sprintf("%s=%d", minTag, int64(minValue)))
		} else if isFloat {
			validationTags = append(validationTags, fmt.Sprintf("%s=%g", minTag, minValue))
		}
	}

	maxValue := float64(0)
	if schema.Maximum != nil {
		maxTag := "lte"
		maxValue = *schema.Maximum
		if schema.ExclusiveMaximum != nil {
			if schema.ExclusiveMaximum.IsA() && schema.ExclusiveMaximum.A {
				maxTag = "lt"
			} else if schema.ExclusiveMaximum.IsB() {
				maxTag = "lt"
				maxValue = schema.ExclusiveMaximum.B
			}
		}
		if isInt {
			validationTags = append(validationTags, fmt.Sprintf("%s=%d", maxTag, int64(maxValue)))
		} else if isFloat {
			validationTags = append(validationTags, fmt.Sprintf("%s=%g", maxTag, maxValue))
		}
	}

	minLength := int64(0)
	if schema.MinLength != nil {
		minLength = *schema.MinLength
		validationTags = append(validationTags, fmt.Sprintf("min=%d", minLength))
	}

	maxLength := int64(0)
	if schema.MaxLength != nil {
		maxLength = *schema.MaxLength
		validationTags = append(validationTags, fmt.Sprintf("max=%d", maxLength))
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

	return Constraints{
		Nullable:       nullable,
		Required:       required,
		ReadOnly:       readOnly,
		WriteOnly:      writeOnly,
		Min:            minValue,
		Max:            maxValue,
		MinLength:      minLength,
		MaxLength:      maxLength,
		ValidationTags: validationTags,
	}
}
