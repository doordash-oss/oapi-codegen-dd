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
	var validationsTags []string

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

	if required && nullable {
		nullable = true
	}
	if required {
		validationsTags = append(validationsTags, "required")
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
			validationsTags = append(validationsTags, fmt.Sprintf("%s=%d", minTag, int64(minValue)))
		} else if isFloat {
			validationsTags = append(validationsTags, fmt.Sprintf("%s=%g", minTag, minValue))
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
			validationsTags = append(validationsTags, fmt.Sprintf("%s=%d", maxTag, int64(maxValue)))
		} else if isFloat {
			validationsTags = append(validationsTags, fmt.Sprintf("%s=%g", maxTag, maxValue))
		}
	}

	minLength := int64(0)
	if schema.MinLength != nil {
		minLength = *schema.MinLength
		validationsTags = append(validationsTags, fmt.Sprintf("min=%d", minLength))
	}

	maxLength := int64(0)
	if schema.MaxLength != nil {
		maxLength = *schema.MaxLength
		validationsTags = append(validationsTags, fmt.Sprintf("max=%d", maxLength))
	}

	// place required first in the list, then sort the rest
	sort.Slice(validationsTags, func(i, j int) bool {
		a, b := validationsTags[i], validationsTags[j]
		if a == "required" || b == "required" {
			return a == "required"
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
		ValidationTags: validationsTags,
	}
}
