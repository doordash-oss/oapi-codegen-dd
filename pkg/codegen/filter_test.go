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

	t.Run("examples removed", func(t *testing.T) {
		contents, err := os.ReadFile("testdata/with-examples.yml")
		require.NoError(t, err)

		opts := Configuration{
			PackageName: packageName,
			Output: &Output{
				UseSingleFile: true,
			},
		}

		// Run our code generation:
		code, err := Generate(contents, opts)
		require.NoError(t, err)
		assert.NotEmpty(t, code)

		combined := code.GetCombined()
		assert.Contains(t, combined, `type ExtraA = string`)
		assert.Contains(t, combined, `type ExtraB = string`)
	})
}
