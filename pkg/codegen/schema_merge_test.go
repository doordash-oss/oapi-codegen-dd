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

func TestMergeOpenapiSchemas(t *testing.T) {
	t.Run("merge nil schema with object schema", func(t *testing.T) {
		contents, err := os.ReadFile("testdata/merge-nil-with-object.yml")
		require.NoError(t, err)

		opts := Configuration{
			PackageName: "testpkg",
			Output: &Output{
				UseSingleFile: true,
			},
		}

		// This should not error - merging nil type with object type is valid
		code, err := Generate(contents, opts)
		require.NoError(t, err)
		assert.NotEmpty(t, code)

		combined := code.GetCombined()
		// Should generate a merged type with properties from both schemas
		assert.Contains(t, combined, "type MergedObject struct")
		assert.Contains(t, combined, "Name")
		assert.Contains(t, combined, "Age")
	})

	t.Run("merge object with nil schema", func(t *testing.T) {
		contents, err := os.ReadFile("testdata/merge-object-with-nil.yml")
		require.NoError(t, err)

		opts := Configuration{
			PackageName: "testpkg",
			Output: &Output{
				UseSingleFile: true,
			},
		}

		code, err := Generate(contents, opts)
		require.NoError(t, err)
		assert.NotEmpty(t, code)

		combined := code.GetCombined()
		assert.Contains(t, combined, "type MergedObject struct")
		assert.Contains(t, combined, "Name")
	})

	t.Run("merge two object schemas", func(t *testing.T) {
		contents, err := os.ReadFile("testdata/merge-two-objects.yml")
		require.NoError(t, err)

		opts := Configuration{
			PackageName: "testpkg",
			Output: &Output{
				UseSingleFile: true,
			},
		}

		code, err := Generate(contents, opts)
		require.NoError(t, err)
		assert.NotEmpty(t, code)

		combined := code.GetCombined()
		assert.Contains(t, combined, "type MergedObject struct")
		assert.Contains(t, combined, "Name")
		assert.Contains(t, combined, "Age")
		assert.Contains(t, combined, "Email")
	})

	t.Run("merge incompatible types should error", func(t *testing.T) {
		contents, err := os.ReadFile("testdata/merge-incompatible-types.yml")
		require.NoError(t, err)

		opts := Configuration{
			PackageName: "testpkg",
			Output: &Output{
				UseSingleFile: true,
			},
		}

		// This should error - can't merge object with array
		_, err = Generate(contents, opts)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "can not merge incompatible types")
	})

	t.Run("merge empty schemas", func(t *testing.T) {
		contents, err := os.ReadFile("testdata/merge-empty-schemas.yml")
		require.NoError(t, err)

		opts := Configuration{
			PackageName: "testpkg",
			Output: &Output{
				UseSingleFile: true,
			},
		}

		code, err := Generate(contents, opts)
		require.NoError(t, err)
		assert.NotEmpty(t, code)
	})
}
