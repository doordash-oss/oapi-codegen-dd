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

func TestSingleElementUnionOptimization(t *testing.T) {
	t.Run("single anyOf with ref should use type directly", func(t *testing.T) {
		doc := loadUnionDocument(t)

		// Get the Order schema which has client: anyOf: [Identity]
		orderSchema := doc.Components.Schemas.Value("Order")
		require.NotNil(t, orderSchema)

		res, err := GenerateGoSchema(orderSchema, ParseOptions{currentTypes: map[string]TypeDefinition{}}.WithPath([]string{"Order"}))
		require.NoError(t, err)

		// Should have a client property
		require.NotEmpty(t, res.Properties)
		var clientProp *Property
		for i := range res.Properties {
			if res.Properties[i].GoName == "Client" {
				clientProp = &res.Properties[i]
				break
			}
		}
		require.NotNil(t, clientProp, "Client property should exist")

		// Client should be a direct reference to Identity, not a wrapper
		// When it's a property with single anyOf, it gets the GoType directly
		assert.Equal(t, "Identity", clientProp.Schema.GoType)

		// Should not have created any union wrapper types for single-element anyOf
		hasAnyOfWrapper := false
		for _, td := range res.AdditionalTypes {
			if td.Name == "Order_Client_AnyOf" || td.Name == "Order_Client" {
				hasAnyOfWrapper = true
				break
			}
		}
		assert.False(t, hasAnyOfWrapper, "Should not create wrapper types for single-element anyOf")
	})

	t.Run("single oneOf with ref should use type directly", func(t *testing.T) {
		doc := loadUnionDocument(t)

		// Get the Verification schema which has verifier: oneOf: [Address]
		verificationSchema := doc.Components.Schemas.Value("Verification")
		require.NotNil(t, verificationSchema)

		res, err := GenerateGoSchema(verificationSchema, ParseOptions{currentTypes: map[string]TypeDefinition{}}.WithPath([]string{"Verification"}))
		require.NoError(t, err)

		// Should have a verifier property
		require.NotEmpty(t, res.Properties)
		var verifierProp *Property
		for i := range res.Properties {
			if res.Properties[i].GoName == "Verifier" {
				verifierProp = &res.Properties[i]
				break
			}
		}
		require.NotNil(t, verifierProp, "Verifier property should exist")

		// Verifier should be a direct reference to Address, not a wrapper
		// When it's a property with single oneOf, it gets the GoType directly
		assert.Equal(t, "Address", verifierProp.Schema.GoType)

		// Should not have created any union wrapper types for single-element oneOf
		hasOneOfWrapper := false
		for _, td := range res.AdditionalTypes {
			if td.Name == "Verification_Verifier_OneOf" || td.Name == "Verification_Verifier" {
				hasOneOfWrapper = true
				break
			}
		}
		assert.False(t, hasOneOfWrapper, "Should not create wrapper types for single-element oneOf")
	})

	t.Run("single anyOf with primitive should use type directly", func(t *testing.T) {
		doc := loadUnionDocument(t)

		// Get the response from /one-of-1 which has anyOf: [string]
		pathItem := doc.Paths.PathItems.Value("/one-of-1")
		require.NotNil(t, pathItem)

		op := pathItem.GetOperations().Value("get")
		require.NotNil(t, op)

		resp := op.Responses.Codes.Value("200")
		require.NotNil(t, resp)

		mediaType := resp.Content.Value("application/json")
		require.NotNil(t, mediaType)

		res, err := GenerateGoSchema(mediaType.Schema, ParseOptions{currentTypes: map[string]TypeDefinition{}}.WithPath([]string{"Response"}))
		require.NoError(t, err)

		// Should be a direct reference to User, not a wrapper
		assert.Equal(t, "User", res.GoType)
		assert.True(t, res.DefineViaAlias)
	})

	t.Run("multiple anyOf elements should still create union", func(t *testing.T) {
		doc := loadUnionDocument(t)

		// Get a schema with multiple anyOf elements
		pathItem := doc.Paths.PathItems.Value("/any-of-2")
		if pathItem == nil {
			t.Skip("Test schema doesn't have /any-of-2 endpoint")
			return
		}

		op := pathItem.GetOperations().Value("get")
		require.NotNil(t, op)

		resp := op.Responses.Codes.Value("200")
		require.NotNil(t, resp)

		mediaType := resp.Content.Value("application/json")
		require.NotNil(t, mediaType)

		res, err := GenerateGoSchema(mediaType.Schema, ParseOptions{currentTypes: map[string]TypeDefinition{}}.WithPath([]string{"Response"}))
		require.NoError(t, err)

		// Should have created a wrapper with union
		assert.Contains(t, res.GoType, "struct")
		assert.NotEmpty(t, res.AdditionalTypes)
	})

	t.Run("multiple oneOf elements should still create union", func(t *testing.T) {
		doc := loadUnionDocument(t)

		// Get the response from /one-of-2 which has oneOf: [User, string]
		pathItem := doc.Paths.PathItems.Value("/one-of-2")
		require.NotNil(t, pathItem)

		op := pathItem.GetOperations().Value("get")
		require.NotNil(t, op)

		resp := op.Responses.Codes.Value("200")
		require.NotNil(t, resp)

		mediaType := resp.Content.Value("application/json")
		require.NotNil(t, mediaType)

		res, err := GenerateGoSchema(mediaType.Schema, ParseOptions{currentTypes: map[string]TypeDefinition{}}.WithPath([]string{"Response"}))
		require.NoError(t, err)

		// Should have created a wrapper with union (Either type for 2 elements)
		assert.Contains(t, res.GoType, "struct")
		assert.NotEmpty(t, res.AdditionalTypes)

		// The union type should be an Either
		unionType := res.AdditionalTypes[0]
		assert.Contains(t, unionType.Schema.GoType, "Either")
	})
}

func TestNullableUnionOptimization(t *testing.T) {
	t.Run("anyOf with null and type should create nullable property", func(t *testing.T) {
		contents := []byte(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Test
paths:
  /users:
    get:
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/User'
components:
  schemas:
    User:
      type: object
      properties:
        name:
          anyOf:
            - type: string
            - type: "null"
`)
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
		// Should generate a nullable string property, not a union
		assert.Contains(t, combined, "Name *string")
		// Should NOT contain union/Either types
		assert.NotContains(t, combined, "Either")
		assert.NotContains(t, combined, "AnyOf")
	})

	t.Run("oneOf with null and ref should create nullable property", func(t *testing.T) {
		contents := []byte(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Test
paths:
  /users:
    get:
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/User'
components:
  schemas:
    Address:
      type: object
      properties:
        city:
          type: string
    User:
      type: object
      properties:
        address:
          oneOf:
            - $ref: '#/components/schemas/Address'
            - type: "null"
`)
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
		// Should generate a nullable Address property, not a union
		assert.Contains(t, combined, "Address *Address")
		// Should NOT contain union/Either types
		assert.NotContains(t, combined, "Either")
		assert.NotContains(t, combined, "OneOf")
	})

	t.Run("anyOf with null first and type second", func(t *testing.T) {
		contents := []byte(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Test
paths:
  /users:
    get:
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/User'
components:
  schemas:
    User:
      type: object
      properties:
        age:
          anyOf:
            - type: "null"
            - type: integer
`)
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
		// Should generate a nullable int property
		assert.Contains(t, combined, "Age *int")
		// Should NOT contain union types
		assert.NotContains(t, combined, "Either")
		assert.NotContains(t, combined, "AnyOf")
	})

	t.Run("anyOf with two non-null types should create union", func(t *testing.T) {
		contents := []byte(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Test
paths:
  /users:
    get:
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/User'
components:
  schemas:
    User:
      type: object
      properties:
        value:
          anyOf:
            - type: string
            - type: integer
`)
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
		// Should create a union type (Either) for two different types
		assert.Contains(t, combined, "Value")
		assert.Contains(t, combined, "Either")
	})
}
