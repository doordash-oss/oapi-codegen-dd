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
				"gte=10",
				"lt=100",
			},
		}, res)
	})
}
