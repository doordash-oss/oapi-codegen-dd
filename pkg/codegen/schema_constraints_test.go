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
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/stretchr/testify/assert"
)

func TestNewConstraints(t *testing.T) {
	t.Run("integer constraints", func(t *testing.T) {
		minValue := float64(10)
		maxValue := float64(100)
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
			Required: true,
			Min:      minValue,
			Max:      float64(99),
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
			Min:      minValue,
			Max:      float64(100),
			Nullable: true,
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
			MaxLength: 100,
			Nullable:  true,
			ValidationTags: []string{
				"omitempty",
				"max=100",
			},
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

		assert.Equal(t, Constraints{
			Required: false,
		}, res)
	})
}
