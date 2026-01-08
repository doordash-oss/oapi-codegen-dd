package expression

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExpressionCircularReference tests that the Expression type is properly generated
// with all properties including the circular reference in the Not property
func TestExpressionCircularReference(t *testing.T) {
	t.Run("Expression has all properties", func(t *testing.T) {
		// Create an Expression with all properties
		expr := Expression{
			Or: Expressions{
				Expression{
					Dimensions: &DimensionValues{
						Key:    strPtr("service"),
						Values: []string{"EC2", "S3"},
					},
				},
			},
			And: Expressions{
				Expression{
					Dimensions: &DimensionValues{
						Key:    strPtr("region"),
						Values: []string{"us-east-1"},
					},
				},
			},
			Not: &Expression{
				Dimensions: &DimensionValues{
					Key:    strPtr("environment"),
					Values: []string{"test"},
				},
			},
			Dimensions: &DimensionValues{
				Key:    strPtr("account"),
				Values: []string{"123456789"},
			},
		}

		// Verify all properties are set
		assert.NotNil(t, expr.Or)
		assert.NotNil(t, expr.And)
		assert.NotNil(t, expr.Not)
		assert.NotNil(t, expr.Dimensions)
	})

	t.Run("Expression with nested Not", func(t *testing.T) {
		// Create an Expression with nested Not (circular reference)
		expr := Expression{
			Not: &Expression{
				Not: &Expression{
					Dimensions: &DimensionValues{
						Key:    strPtr("service"),
						Values: []string{"EC2"},
					},
				},
			},
		}

		// Verify nested structure
		assert.NotNil(t, expr.Not)
		assert.NotNil(t, expr.Not.Not)
		assert.NotNil(t, expr.Not.Not.Dimensions)
		assert.Equal(t, "service", *expr.Not.Not.Dimensions.Key)
	})

	t.Run("Expressions array contains Expression", func(t *testing.T) {
		// Create an Expressions array
		exprs := Expressions{
			Expression{
				Dimensions: &DimensionValues{
					Key:    strPtr("service"),
					Values: []string{"EC2"},
				},
			},
			Expression{
				Not: &Expression{
					Dimensions: &DimensionValues{
						Key:    strPtr("region"),
						Values: []string{"us-west-2"},
					},
				},
			},
		}

		// Verify array structure
		assert.Len(t, exprs, 2)
		assert.NotNil(t, exprs[0].Dimensions)
		assert.NotNil(t, exprs[1].Not)
	})

	t.Run("FilterRequest with Expression", func(t *testing.T) {
		// Create a FilterRequest
		req := FilterRequest{
			Filter: &Expression{
				Or: Expressions{
					Expression{
						Dimensions: &DimensionValues{
							Key:    strPtr("service"),
							Values: []string{"EC2"},
						},
					},
				},
			},
		}

		// Verify structure
		assert.NotNil(t, req.Filter)
		assert.NotNil(t, req.Filter.Or)
		assert.Len(t, req.Filter.Or, 1)
	})
}

func strPtr(s string) *string {
	return &s
}
