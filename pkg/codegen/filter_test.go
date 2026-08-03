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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterOperationsByTag(t *testing.T) {
	packageName := "testswagger"
	t.Run("include tags", func(t *testing.T) {
		cfg := Configuration{
			PackageName: packageName,
			Filter: FilterConfig{
				Include: FilterParamsConfig{
					Tags: []string{"hippo", "giraffe", "cat"},
				},
			},
			Output: &Output{
				UseSingleFile: true,
			},
		}

		// Run our code generation:
		code, err := Generate([]byte(testDocument), cfg)
		require.NoError(t, err)
		assert.NotEmpty(t, code)

		assert.Contains(t, code.GetCombined(), `type CatDeadCause string`)
	})

	t.Run("exclude tags", func(t *testing.T) {
		opts := Configuration{
			PackageName: packageName,
			Filter: FilterConfig{
				Exclude: FilterParamsConfig{
					Tags: []string{"hippo", "giraffe", "cat"},
				},
			},
			Output: &Output{
				UseSingleFile: true,
			},
		}

		// Run our code generation:
		code, err := Generate([]byte(testDocument), opts)
		require.NoError(t, err)
		assert.NotEmpty(t, code)
		assert.NotContains(t, code.GetCombined(), `"/cat"`)
	})
}

func TestFilterOperationsByOperationID(t *testing.T) {
	packageName := "testswagger"

	t.Run("include operation ids", func(t *testing.T) {
		opts := Configuration{
			PackageName: packageName,
			Filter: FilterConfig{
				Include: FilterParamsConfig{
					OperationIDs: []string{"getCatStatus"},
				},
			},
			Output: &Output{
				UseSingleFile: true,
			},
		}

		// Run our code generation:
		code, err := Generate([]byte(testDocument), opts)
		require.NoError(t, err)
		assert.NotEmpty(t, code)
		assert.Contains(t, code.GetCombined(), `type CatDeadCause string`)
	})

	t.Run("exclude operation ids", func(t *testing.T) {
		opts := Configuration{
			PackageName: packageName,
			Filter: FilterConfig{
				Exclude: FilterParamsConfig{
					OperationIDs: []string{"getCatStatus"},
				},
			},
		}

		// Run our code generation:
		code, err := Generate([]byte(testDocument), opts)
		require.NoError(t, err)
		assert.NotEmpty(t, code)
		assert.NotContains(t, code.GetCombined(), `"/cat"`)
	})

	t.Run("empty filter does not remove operations", func(t *testing.T) {
		opts := Configuration{
			PackageName: packageName,
			Filter: FilterConfig{
				Include: FilterParamsConfig{
					OperationIDs: []string{},
				},
				Exclude: FilterParamsConfig{
					OperationIDs: []string{},
				},
			},
			Generate: &GenerateOptions{
				Client: true,
			},
			Output: &Output{
				UseSingleFile: true,
			},
		}

		// Run our code generation:
		code, err := Generate([]byte(testDocument), opts)
		require.NoError(t, err)
		assert.NotEmpty(t, code)

		// Should contain operations since empty filters don't remove anything
		combined := code.GetCombined()
		assert.Contains(t, combined, `type CatDeadCause string`)
		assert.Contains(t, combined, `"/cat"`)
	})

	t.Run("nil filter does not remove operations", func(t *testing.T) {
		opts := Configuration{
			PackageName: packageName,
			Filter: FilterConfig{
				Include: FilterParamsConfig{
					OperationIDs: nil,
				},
				Exclude: FilterParamsConfig{
					OperationIDs: nil,
				},
			},
			Generate: &GenerateOptions{
				Client: true,
			},
			Output: &Output{
				UseSingleFile: true,
			},
		}

		// Run our code generation:
		code, err := Generate([]byte(testDocument), opts)
		require.NoError(t, err)
		assert.NotEmpty(t, code)

		// Should contain operations since nil filters don't remove anything
		combined := code.GetCombined()
		assert.Contains(t, combined, `type CatDeadCause string`)
		assert.Contains(t, combined, `"/cat"`)
	})

	t.Run("default configuration does not filter operations", func(t *testing.T) {
		opts := Configuration{
			PackageName: packageName,
			// Filter is zero value - should not filter anything
			Generate: &GenerateOptions{
				Client: true,
			},
			Output: &Output{
				UseSingleFile: true,
			},
		}

		// Run our code generation:
		code, err := Generate([]byte(testDocument), opts)
		require.NoError(t, err)
		assert.NotEmpty(t, code)

		// Should contain all operations
		combined := code.GetCombined()
		assert.Contains(t, combined, `type CatDeadCause string`)
		assert.Contains(t, combined, `"/cat"`)
	})
}

func TestFilterOperationsByPath(t *testing.T) {
	packageName := "testswagger"

	t.Run("include paths", func(t *testing.T) {
		opts := Configuration{
			PackageName: packageName,
			Filter: FilterConfig{
				Include: FilterParamsConfig{
					Paths: []string{"/cat"},
				},
			},
			Generate: &GenerateOptions{
				Client: true,
			},
			Output: &Output{
				UseSingleFile: true,
			},
		}

		code, err := Generate([]byte(testDocument), opts)
		require.NoError(t, err)
		assert.NotEmpty(t, code)

		combined := code.GetCombined()
		assert.Contains(t, combined, `"/cat"`)
		assert.NotContains(t, combined, `"/test/{name}"`)
		assert.NotContains(t, combined, `"/enum"`)
	})

	t.Run("exclude paths", func(t *testing.T) {
		opts := Configuration{
			PackageName: packageName,
			Filter: FilterConfig{
				Exclude: FilterParamsConfig{
					Paths: []string{"/cat", "/enum"},
				},
			},
			Generate: &GenerateOptions{
				Client: true,
			},
			Output: &Output{
				UseSingleFile: true,
			},
		}

		code, err := Generate([]byte(testDocument), opts)
		require.NoError(t, err)
		assert.NotEmpty(t, code)

		combined := code.GetCombined()
		assert.NotContains(t, combined, `"/cat"`)
		assert.NotContains(t, combined, `"/enum"`)
		assert.Contains(t, combined, `"/test/{name}"`)
	})

	t.Run("empty include paths does not filter", func(t *testing.T) {
		opts := Configuration{
			PackageName: packageName,
			Filter: FilterConfig{
				Include: FilterParamsConfig{
					Paths: []string{}, // Empty - should not filter
				},
			},
			Generate: &GenerateOptions{
				Client: true,
			},
			Output: &Output{
				UseSingleFile: true,
			},
		}

		code, err := Generate([]byte(testDocument), opts)
		require.NoError(t, err)
		assert.NotEmpty(t, code)

		combined := code.GetCombined()
		assert.Contains(t, combined, `"/cat"`)
		assert.Contains(t, combined, `"/test/{name}"`)
		assert.Contains(t, combined, `"/enum"`)
	})

	t.Run("empty exclude paths does not filter", func(t *testing.T) {
		opts := Configuration{
			PackageName: packageName,
			Filter: FilterConfig{
				Exclude: FilterParamsConfig{
					Paths: []string{}, // Empty - should not filter
				},
			},
			Generate: &GenerateOptions{
				Client: true,
			},
			Output: &Output{
				UseSingleFile: true,
			},
		}

		code, err := Generate([]byte(testDocument), opts)
		require.NoError(t, err)
		assert.NotEmpty(t, code)

		combined := code.GetCombined()
		assert.Contains(t, combined, `"/cat"`)
		assert.Contains(t, combined, `"/test/{name}"`)
		assert.Contains(t, combined, `"/enum"`)
	})

	t.Run("nil include paths does not filter", func(t *testing.T) {
		opts := Configuration{
			PackageName: packageName,
			Filter: FilterConfig{
				Include: FilterParamsConfig{
					Paths: nil, // Nil - should not filter
				},
			},
			Generate: &GenerateOptions{
				Client: true,
			},
			Output: &Output{
				UseSingleFile: true,
			},
		}

		code, err := Generate([]byte(testDocument), opts)
		require.NoError(t, err)
		assert.NotEmpty(t, code)

		combined := code.GetCombined()
		assert.Contains(t, combined, `"/cat"`)
		assert.Contains(t, combined, `"/test/{name}"`)
		assert.Contains(t, combined, `"/enum"`)
	})

	t.Run("nil exclude paths does not filter", func(t *testing.T) {
		opts := Configuration{
			PackageName: packageName,
			Filter: FilterConfig{
				Exclude: FilterParamsConfig{
					Paths: nil, // Nil - should not filter
				},
			},
			Generate: &GenerateOptions{
				Client: true,
			},
			Output: &Output{
				UseSingleFile: true,
			},
		}

		code, err := Generate([]byte(testDocument), opts)
		require.NoError(t, err)
		assert.NotEmpty(t, code)

		combined := code.GetCombined()
		assert.Contains(t, combined, `"/cat"`)
		assert.Contains(t, combined, `"/test/{name}"`)
		assert.Contains(t, combined, `"/enum"`)
	})
}

func TestFilterWebhooks(t *testing.T) {
	contents, err := os.ReadFile("testdata/filter-webhooks.yml")
	require.NoError(t, err)

	t.Run("include webhooks", func(t *testing.T) {
		doc, err := LoadDocumentFromContents(contents)
		require.NoError(t, err)

		model, err := doc.BuildV3Model()
		require.NoError(t, err)

		cfg := FilterConfig{
			Include: FilterParamsConfig{
				Webhooks: []string{"order.created"},
			},
		}

		removed := filterOperations(&model.Model, cfg)
		assert.True(t, removed)

		// order.created should remain
		_, hasOrder := model.Model.Webhooks.Get("order.created")
		assert.True(t, hasOrder, "order.created webhook should be included")

		// payment.completed should be removed
		_, hasPayment := model.Model.Webhooks.Get("payment.completed")
		assert.False(t, hasPayment, "payment.completed webhook should be excluded")
	})

	t.Run("exclude webhooks", func(t *testing.T) {
		doc, err := LoadDocumentFromContents(contents)
		require.NoError(t, err)

		model, err := doc.BuildV3Model()
		require.NoError(t, err)

		cfg := FilterConfig{
			Exclude: FilterParamsConfig{
				Webhooks: []string{"payment.completed"},
			},
		}

		removed := filterOperations(&model.Model, cfg)
		assert.True(t, removed)

		// order.created should remain
		_, hasOrder := model.Model.Webhooks.Get("order.created")
		assert.True(t, hasOrder, "order.created webhook should remain")

		// payment.completed should be removed
		_, hasPayment := model.Model.Webhooks.Get("payment.completed")
		assert.False(t, hasPayment, "payment.completed webhook should be excluded")
	})

	t.Run("empty webhook filter does not remove webhooks", func(t *testing.T) {
		doc, err := LoadDocumentFromContents(contents)
		require.NoError(t, err)

		model, err := doc.BuildV3Model()
		require.NoError(t, err)

		cfg := FilterConfig{}
		removed := filterOperations(&model.Model, cfg)
		assert.False(t, removed)

		// Both webhooks should remain
		_, hasOrder := model.Model.Webhooks.Get("order.created")
		assert.True(t, hasOrder)
		_, hasPayment := model.Model.Webhooks.Get("payment.completed")
		assert.True(t, hasPayment)
	})

	t.Run("filter webhook operations by tag", func(t *testing.T) {
		doc, err := LoadDocumentFromContents(contents)
		require.NoError(t, err)

		model, err := doc.BuildV3Model()
		require.NoError(t, err)

		cfg := FilterConfig{
			Include: FilterParamsConfig{
				Tags: []string{"orders"},
			},
		}

		removed := filterOperations(&model.Model, cfg)
		assert.True(t, removed)

		// order.created should still have its post operation
		orderItem, hasOrder := model.Model.Webhooks.Get("order.created")
		assert.True(t, hasOrder)
		assert.NotNil(t, orderItem.Post)

		// payment.completed should have its post operation nil'd out
		paymentItem, hasPayment := model.Model.Webhooks.Get("payment.completed")
		assert.True(t, hasPayment)
		assert.Nil(t, paymentItem.Post)
	})

	t.Run("filter webhook operations by operationID", func(t *testing.T) {
		doc, err := LoadDocumentFromContents(contents)
		require.NoError(t, err)

		model, err := doc.BuildV3Model()
		require.NoError(t, err)

		cfg := FilterConfig{
			Exclude: FilterParamsConfig{
				OperationIDs: []string{"paymentCompleted"},
			},
		}

		removed := filterOperations(&model.Model, cfg)
		assert.True(t, removed)

		// order.created post should remain
		orderItem, _ := model.Model.Webhooks.Get("order.created")
		assert.NotNil(t, orderItem.Post)

		// payment.completed post should be nil'd out
		paymentItem, _ := model.Model.Webhooks.Get("payment.completed")
		assert.Nil(t, paymentItem.Post)
	})
}

func TestGenerateWebhookTypes(t *testing.T) {
	contents, err := os.ReadFile("testdata/webhooks-with-examples.yml")
	require.NoError(t, err)

	cfg := Configuration{
		PackageName: "webhooktest",
		Output: &Output{
			UseSingleFile: true,
		},
	}

	code, err := Generate(contents, cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, code)

	combined := code.GetCombined()
	assert.Contains(t, combined, "PaymentRequest")
	assert.Contains(t, combined, "Response")
}

func TestGenerateWebhookTypesMultiFile(t *testing.T) {
	contents, err := os.ReadFile("testdata/webhooks-with-examples.yml")
	require.NoError(t, err)

	cfg := Configuration{
		PackageName: "webhooktest",
		Output: &Output{
			UseSingleFile: false,
		},
	}

	code, err := Generate(contents, cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, code)

	// Webhook types should be in the "webhooks" output file
	webhooksCode, hasWebhooks := code["webhooks"]
	assert.True(t, hasWebhooks, "should have a 'webhooks' output file for webhook types")
	assert.Contains(t, webhooksCode, "PaymentCreatedBody")
	assert.Contains(t, webhooksCode, "PaymentCreatedResponse")

	// Component schemas should be in the "types" output file
	typesCode, hasTypes := code["types"]
	assert.True(t, hasTypes, "should have a 'types' output file for component schemas")
	assert.Contains(t, typesCode, "PaymentRequest")
}
