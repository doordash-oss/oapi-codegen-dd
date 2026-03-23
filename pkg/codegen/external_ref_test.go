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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExternalFileRefResolution(t *testing.T) {
	testdataDir, err := filepath.Abs("testdata")
	require.NoError(t, err)

	specPath := filepath.Join(testdataDir, "external-ref-api.yaml")
	contents, err := os.ReadFile(specPath)
	require.NoError(t, err)

	t.Run("fails without BasePath", func(t *testing.T) {
		cfg := NewDefaultConfiguration()
		cfg.BasePath = ""

		_, err := CreateDocument(contents, cfg)
		// Without BasePath, external refs cannot be resolved
		assert.Error(t, err)
	})

	t.Run("succeeds with BasePath", func(t *testing.T) {
		cfg := NewDefaultConfiguration()
		cfg.BasePath = testdataDir

		doc, err := CreateDocument(contents, cfg)
		require.NoError(t, err)

		model, errs := doc.BuildV3Model()
		require.Empty(t, errs)

		// Verify the external ref was resolved — the Address schema should be present
		getUserResp := model.Model.Paths.PathItems.GetOrZero("/users/{id}").Get.Responses.Codes.GetOrZero("200")
		require.NotNil(t, getUserResp)

		jsonContent := getUserResp.Content.GetOrZero("application/json")
		require.NotNil(t, jsonContent)

		schema := jsonContent.Schema.Schema()
		require.NotNil(t, schema)

		addressProp := schema.Properties.GetOrZero("address")
		require.NotNil(t, addressProp)

		// The resolved Address schema should have street and city properties
		resolvedSchema := addressProp.Schema()
		require.NotNil(t, resolvedSchema)
		assert.True(t, resolvedSchema.Properties.Len() > 0, "Address schema should have properties after ref resolution")

		streetProp := resolvedSchema.Properties.GetOrZero("street")
		assert.NotNil(t, streetProp, "Address should have a 'street' property")

		cityProp := resolvedSchema.Properties.GetOrZero("city")
		assert.NotNil(t, cityProp, "Address should have a 'city' property")
	})

	t.Run("end-to-end code generation with external refs", func(t *testing.T) {
		cfg := NewDefaultConfiguration()
		cfg.BasePath = testdataDir

		code, err := Generate(contents, cfg)
		require.NoError(t, err)
		assert.NotEmpty(t, code)

		combined := code.GetCombined()
		assert.Contains(t, combined, "Address")
		assert.Contains(t, combined, "Street")
		assert.Contains(t, combined, "City")
	})
}
