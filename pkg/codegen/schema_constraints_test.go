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
	"os"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/stretchr/testify/assert"
)

func TestNewConstraints(t *testing.T) {
	t.Run("integer constraints", func(t *testing.T) {
		minValue := float64(10)
		maxValue := float64(99)
		schema := &base.Schema{
			Type:     []string{"integer"},
			Format:   "int32",
			Minimum:  &minValue,
			Maximum:  &maxValue,
			Required: []string{"foo"},
			ExclusiveMaximum: &base.DynamicValue[bool, float64]{
				N: 1,
				B: 99,
			},
		}

		res := newConstraints(schema, ConstraintsContext{
			name:       "foo",
			hasNilType: false,
			required:   true,
		})

		assert.Equal(t, Constraints{
			Required: ptr(true),
			Min:      &minValue,
			Max:      &maxValue,
			ValidationTags: []string{
				"required",
				"gte=10",
				"lt=99",
			},
		}, res)
	})

	t.Run("number constraints", func(t *testing.T) {
		minValue := float64(10)
		maxValue := float64(100)
		schema := &base.Schema{
			Type:    []string{"number"},
			Minimum: &minValue,
			Maximum: &maxValue,
			ExclusiveMaximum: &base.DynamicValue[bool, float64]{
				N: 0,
				A: true,
			},
		}

		res := newConstraints(schema, ConstraintsContext{
			name: "foo",
		})

		assert.Equal(t, Constraints{
			Min:      &minValue,
			Max:      &maxValue,
			Nullable: ptr(true),
			ValidationTags: []string{
				"omitempty",
				"gte=10",
				"lt=100",
			},
		}, res)
	})

	t.Run("optional string with max length", func(t *testing.T) {
		maxLn := int64(100)
		schema := &base.Schema{
			Type:      []string{"string"},
			MaxLength: &maxLn,
		}

		res := newConstraints(schema, ConstraintsContext{})

		assert.Equal(t, Constraints{
			MaxLength: &maxLn,
			Nullable:  ptr(true),
			ValidationTags: []string{
				"omitempty",
				"max=100",
			},
		}, res)
	})

	t.Run("string with pattern", func(t *testing.T) {
		pattern := "^[0-9]{4,7}$"
		schema := &base.Schema{
			Type:    []string{"string"},
			Pattern: pattern,
		}

		res := newConstraints(schema, ConstraintsContext{})

		assert.Equal(t, Constraints{
			Pattern:  &pattern,
			Nullable: ptr(true),
			// ValidationTags is nil because "omitempty" alone gets cleared
		}, res)
	})

	t.Run("boolean type", func(t *testing.T) {
		schema := &base.Schema{
			Type:     []string{"boolean"},
			Required: []string{"foo"},
		}

		res := newConstraints(schema, ConstraintsContext{
			name:       "foo",
			hasNilType: false,
			required:   true,
		})

		// For boolean types, required is set to false to avoid validation failures with false values
		// Since required=false, we don't set the Required pointer (it remains nil)
		assert.Equal(t, Constraints{}, res)
	})

	t.Run("array with minItems and maxItems", func(t *testing.T) {
		minItems := int64(1)
		maxItems := int64(10)
		schema := &base.Schema{
			Type:     []string{"array"},
			MinItems: &minItems,
			MaxItems: &maxItems,
		}

		res := newConstraints(schema, ConstraintsContext{})

		assert.Equal(t, Constraints{
			MinItems: &minItems,
			MaxItems: &maxItems,
			Nullable: ptr(true),
		}, res)
	})

	t.Run("object with minProperties and maxProperties", func(t *testing.T) {
		minProps := int64(1)
		maxProps := int64(5)
		schema := &base.Schema{
			Type:          []string{"object"},
			MinProperties: &minProps,
			MaxProperties: &maxProps,
		}

		res := newConstraints(schema, ConstraintsContext{})

		assert.Equal(t, Constraints{
			MinProperties: &minProps,
			MaxProperties: &maxProps,
			Nullable:      ptr(true),
		}, res)
	})

	t.Run("integer with maxLength - invalid spec, should be ignored", func(t *testing.T) {
		maxLn := int64(8)
		schema := &base.Schema{
			Type:      []string{"integer"},
			Format:    "int32",
			MaxLength: &maxLn,
		}

		res := newConstraints(schema, ConstraintsContext{})

		// maxLength on integer is invalid per OpenAPI spec
		// We should NOT store it at all
		assert.Equal(t, Constraints{
			Nullable: ptr(true),
		}, res)
	})

	t.Run("number with minLength and maxLength - invalid spec, should be ignored", func(t *testing.T) {
		minLn := int64(2)
		maxLn := int64(11)
		schema := &base.Schema{
			Type:      []string{"number"},
			Format:    "float",
			MinLength: &minLn,
			MaxLength: &maxLn,
		}

		res := newConstraints(schema, ConstraintsContext{})

		// minLength/maxLength on number is invalid per OpenAPI spec
		// We should NOT store them at all
		assert.Equal(t, Constraints{
			Nullable: ptr(true),
		}, res)
	})

	t.Run("string with minimum and maximum - invalid spec, should be ignored", func(t *testing.T) {
		minVal := float64(10)
		maxVal := float64(100)
		schema := &base.Schema{
			Type:    []string{"string"},
			Minimum: &minVal,
			Maximum: &maxVal,
		}

		res := newConstraints(schema, ConstraintsContext{})

		// minimum/maximum on string is invalid per OpenAPI spec
		// We should NOT store them at all
		assert.Equal(t, Constraints{
			Nullable: ptr(true),
		}, res)
	})
}

func TestInvalidConstraintsCodegen(t *testing.T) {
	t.Run("integer and number with minLength/maxLength should not fail", func(t *testing.T) {
		spec, err := os.ReadFile("testdata/invalid-constraints.yaml")
		assert.NoError(t, err)

		opts := Configuration{
			PackageName: "testpkg",
			Output: &Output{
				UseSingleFile: true,
			},
		}

		code, err := Generate(spec, opts)
		assert.NoError(t, err)
		assert.NotEmpty(t, code)

		combined := code.GetCombined()

		// Verify integer field has minimum/maximum validation but NOT minLength/maxLength
		assert.Contains(t, combined, "Age")
		assert.Contains(t, combined, `validate:"omitempty,gte=18,lte=99"`)

		// Verify number field has minimum/maximum validation but NOT minLength/maxLength
		assert.Contains(t, combined, "Price")
		assert.Contains(t, combined, `validate:"omitempty,gte=0.01,lte=999.99"`)

		// Verify boolean field has no validation tags (minLength/maxLength ignored)
		assert.Contains(t, combined, "Active")
		assert.Contains(t, combined, `Active`)
		assert.NotContains(t, combined, `Active *bool    `+"`json:\"active,omitempty\" validate:") // No validate tag

		// Verify string field HAS minLength/maxLength validation
		assert.Contains(t, combined, "Name")
		assert.Contains(t, combined, `validate:"omitempty,max=50,min=2"`)

		// Verify string field with invalid minimum/maximum has only minLength/maxLength
		assert.Contains(t, combined, "Description")
		assert.Contains(t, combined, `validate:"omitempty,max=200,min=5"`)
	})
}
