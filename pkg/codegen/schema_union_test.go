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
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadUnionDocument(t *testing.T) v3.Document {
	t.Helper()
	content, err := os.ReadFile("testdata/unions.yml")
	require.NoError(t, err)

	srcDoc, err := LoadDocumentFromContents(content)
	require.NoError(t, err)

	v3Model, err := srcDoc.BuildV3Model()
	if err != nil {
		t.Fatalf("error building model: %v", err)
	}

	return v3Model.Model
}

func getOperationResponse(t *testing.T, doc v3.Document, path, method string) *base.SchemaProxy {
	t.Helper()
	pathItem := doc.Paths.PathItems.Value(path)
	require.NotNil(t, pathItem, "Path item not found for path: %s", path)

	op := pathItem.GetOperations().Value(method)
	require.NotNil(t, op, "Operation not found for method: %s", method)

	resp := op.Responses.Codes.Value("200")
	require.NotNil(t, resp, "Response not found for status code: 200")

	mediaType := resp.Content.Value("application/json")
	require.NotNil(t, mediaType, "Media type not found for content type: application/json")

	return mediaType.Schema
}

func TestGenerateGoSchema_generateUnion(t *testing.T) {
	t.Run("one-of 1 possible values produces no union", func(t *testing.T) {
		doc := loadUnionDocument(t)
		getUser := getOperationResponse(t, doc, "/one-of-1", "get")

		res, err := GenerateGoSchema(getUser, ParseOptions{currentTypes: map[string]TypeDefinition{}}.WithPath([]string{"User"}))
		require.NoError(t, err)

		// With single element oneOf, it should just return the User type directly
		assert.Equal(t, "User", res.GoType)
		assert.True(t, res.DefineViaAlias)
		assert.Nil(t, res.UnionElements)
		assert.Equal(t, 0, len(res.AdditionalTypes))
	})

	t.Run("one-of 2 possible values", func(t *testing.T) {
		doc := loadUnionDocument(t)
		getUser := getOperationResponse(t, doc, "/one-of-2", "get")

		res, err := GenerateGoSchema(getUser, ParseOptions{currentTypes: map[string]TypeDefinition{}}.WithPath([]string{"User"}))
		require.NoError(t, err)

		assert.Equal(t, "struct {\n    User_OneOf *User_OneOf`json:\"-\"`\n}", res.GoType)
		assert.Equal(t, 1, len(res.AdditionalTypes))
		assert.Equal(t, "User_OneOf", res.AdditionalTypes[0].Name)
		assert.Equal(t, "struct {\nruntime.Either[User, string]\n}", res.AdditionalTypes[0].Schema.GoType)
	})

	t.Run("one-of 3 possible values", func(t *testing.T) {
		doc := loadUnionDocument(t)
		getUser := getOperationResponse(t, doc, "/one-of-3", "get")

		res, err := GenerateGoSchema(getUser, ParseOptions{currentTypes: map[string]TypeDefinition{}}.WithPath([]string{"User"}))
		require.NoError(t, err)

		assert.Equal(t, "struct {\n    User_OneOf *User_OneOf`json:\"-\"`\n}", res.GoType)
		assert.Nil(t, res.UnionElements)
		assert.Equal(t, 1, len(res.AdditionalTypes))
		assert.Equal(t, "User_OneOf", res.AdditionalTypes[0].Name)
		assert.Equal(t, "struct {\nunion json.RawMessage\n}", res.AdditionalTypes[0].Schema.GoType)
		assert.Equal(t, []UnionElement{"User", "Error", "string"}, res.AdditionalTypes[0].Schema.UnionElements)
	})
}

func TestExtractDiscriminatorValue(t *testing.T) {
	t.Run("extracts discriminator value from inline schema with enum", func(t *testing.T) {
		// Create a simple inline schema with a discriminator property that has an enum value
		yamlContent := `
openapi: 3.0.0
info:
  title: Test
  version: 1.0.0
paths:
  /test:
    get:
      operationId: test
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                oneOf:
                  - type: object
                    properties:
                      type:
                        type: string
                        enum:
                          - dog
                      name:
                        type: string
                  - type: object
                    properties:
                      type:
                        type: string
                        enum:
                          - cat
                      lives:
                        type: integer
                discriminator:
                  propertyName: type
`
		srcDoc, err := LoadDocumentFromContents([]byte(yamlContent))
		require.NoError(t, err)

		v3Model, err := srcDoc.BuildV3Model()
		require.NoError(t, err)

		doc := v3Model.Model
		schemaProxy := getOperationResponse(t, doc, "/test", "get")
		require.NotNil(t, schemaProxy)

		schema := schemaProxy.Schema()
		require.NotNil(t, schema)
		require.NotNil(t, schema.OneOf)
		require.Len(t, schema.OneOf, 2)

		// Test extracting discriminator value from first element (dog)
		dogValue := extractDiscriminatorValue(schema.OneOf[0], "type")
		assert.Equal(t, "dog", dogValue)

		// Test extracting discriminator value from second element (cat)
		catValue := extractDiscriminatorValue(schema.OneOf[1], "type")
		assert.Equal(t, "cat", catValue)
	})

	t.Run("returns empty string when discriminator property not found", func(t *testing.T) {
		yamlContent := `
openapi: 3.0.0
info:
  title: Test
  version: 1.0.0
paths:
  /test:
    get:
      operationId: test
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                oneOf:
                  - type: object
                    properties:
                      name:
                        type: string
`
		srcDoc, err := LoadDocumentFromContents([]byte(yamlContent))
		require.NoError(t, err)

		v3Model, err := srcDoc.BuildV3Model()
		require.NoError(t, err)

		doc := v3Model.Model
		schemaProxy := getOperationResponse(t, doc, "/test", "get")
		require.NotNil(t, schemaProxy)

		schema := schemaProxy.Schema()
		require.NotNil(t, schema)
		require.NotNil(t, schema.OneOf)
		require.Len(t, schema.OneOf, 1)

		// Test with non-existent discriminator property
		value := extractDiscriminatorValue(schema.OneOf[0], "type")
		assert.Equal(t, "", value)
	})

	t.Run("returns empty string when property has no enum", func(t *testing.T) {
		yamlContent := `
openapi: 3.0.0
info:
  title: Test
  version: 1.0.0
paths:
  /test:
    get:
      operationId: test
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                oneOf:
                  - type: object
                    properties:
                      type:
                        type: string
`
		srcDoc, err := LoadDocumentFromContents([]byte(yamlContent))
		require.NoError(t, err)

		v3Model, err := srcDoc.BuildV3Model()
		require.NoError(t, err)

		doc := v3Model.Model
		schemaProxy := getOperationResponse(t, doc, "/test", "get")
		require.NotNil(t, schemaProxy)

		schema := schemaProxy.Schema()
		require.NotNil(t, schema)
		require.NotNil(t, schema.OneOf)
		require.Len(t, schema.OneOf, 1)

		// Test with property that has no enum
		value := extractDiscriminatorValue(schema.OneOf[0], "type")
		assert.Equal(t, "", value)
	})
}
