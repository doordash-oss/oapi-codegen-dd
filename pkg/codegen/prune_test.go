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
)

func TestFindReferences(t *testing.T) {
	t.Run("unfiltered", func(t *testing.T) {
		doc, err := LoadDocumentFromContents([]byte(pruneSpecTestFixture))
		assert.NoError(t, err)

		model, _ := doc.BuildV3Model()

		refs := findOperationRefs(&model.Model)
		assert.Len(t, refs, 5)
	})

	t.Run("only cat", func(t *testing.T) {
		doc, err := LoadDocumentFromContents([]byte(pruneSpecTestFixture))
		assert.NoError(t, err)

		model, _ := doc.BuildV3Model()
		m := &model.Model

		cfg := FilterConfig{
			Include: FilterParamsConfig{
				Tags: []string{"cat"},
			},
		}

		filterOperations(m, cfg)

		_, doc2, _, err := doc.RenderAndReload()
		assert.Nil(t, err)
		m2, _ := doc2.BuildV3Model()

		refs := findOperationRefs(&m2.Model)
		assert.Len(t, refs, 3)
	})

	t.Run("only dog", func(t *testing.T) {
		doc, err := LoadDocumentFromContents([]byte(pruneSpecTestFixture))
		assert.NoError(t, err)

		model, _ := doc.BuildV3Model()

		cfg := FilterConfig{
			Include: FilterParamsConfig{
				Tags: []string{"dog"},
			},
		}

		filterOperations(&model.Model, cfg)

		_, doc2, _, err := doc.RenderAndReload()
		assert.Nil(t, err)
		m2, _ := doc2.BuildV3Model()

		refs := findOperationRefs(&m2.Model)
		assert.Len(t, refs, 3)
	})
}

func TestFilterOnlyCat(t *testing.T) {
	// Get a spec from the test definition in this file:
	doc, err := LoadDocumentFromContents([]byte(pruneSpecTestFixture))
	assert.NoError(t, err)

	model, _ := doc.BuildV3Model()

	cfg := FilterConfig{
		Include: FilterParamsConfig{
			Tags: []string{"cat"},
		},
	}

	refs := findOperationRefs(&model.Model)
	assert.Len(t, refs, 5)
	assert.Equal(t, 5, model.Model.Components.Schemas.Len())

	filterOperations(&model.Model, cfg)

	_, doc2, _, err := doc.RenderAndReload()
	assert.Nil(t, err)
	m2, _ := doc2.BuildV3Model()

	refs = findOperationRefs(&m2.Model)
	assert.Len(t, refs, 3)

	assert.NotEmpty(t, m2.Model.Paths.PathItems.GetOrZero("/cat"), "/cat path should still be in spec")
	assert.NotEmpty(t, m2.Model.Paths.PathItems.GetOrZero("/cat").Get, "GET /cat operation should still be in spec")
	assert.Empty(t, m2.Model.Paths.PathItems.GetOrZero("/dog").Get, "GET /dog should have been removed from spec")

	doc, err = pruneSchema(doc2)
	assert.Nil(t, err)
	if err != nil {
		t.Fatal(err)
	}
	model, _ = doc.BuildV3Model()

	assert.Equal(t, 3, model.Model.Components.Schemas.Len())
}

func TestFilterOnlyDog(t *testing.T) {
	// Get a spec from the test definition in this file:
	doc, err := LoadDocumentFromContents([]byte(pruneSpecTestFixture))
	assert.NoError(t, err)

	model, _ := doc.BuildV3Model()
	m := &model.Model

	cfg := FilterConfig{
		Include: FilterParamsConfig{
			Tags: []string{"dog"},
		},
	}

	refs := findOperationRefs(m)
	assert.Len(t, refs, 5)

	filterOperations(m, cfg)

	_, doc2, _, err := doc.RenderAndReload()
	assert.Nil(t, err)
	m2, _ := doc2.BuildV3Model()

	refs = findOperationRefs(&m2.Model)
	assert.Len(t, refs, 3)

	assert.Equal(t, 5, m2.Model.Components.Schemas.Len())

	assert.NotEmpty(t, m2.Model.Paths.PathItems.GetOrZero("/dog"))
	assert.NotEmpty(t, m2.Model.Paths.PathItems.GetOrZero("/dog").Get)
	assert.Empty(t, m2.Model.Paths.PathItems.GetOrZero("/cat").Get)

	doc3, _ := pruneSchema(doc2)
	assert.Nil(t, err)
	if err != nil {
		t.Fatal(err)
	}
	m3, _ := doc3.BuildV3Model()

	assert.Equal(t, 3, m3.Model.Components.Schemas.Len())
}

func TestPruningUnusedComponents(t *testing.T) {
	// Get a spec from the test definition in this file:
	doc, err := LoadDocumentFromContents([]byte(pruneComprehensiveTestFixture))
	assert.NoError(t, err)

	model, _ := doc.BuildV3Model()
	m := &model.Model

	assert.Equal(t, 8, m.Components.Schemas.Len())
	assert.Equal(t, 1, m.Components.Parameters.Len())
	assert.Equal(t, 2, m.Components.SecuritySchemes.Len())
	assert.Equal(t, 1, m.Components.RequestBodies.Len())
	assert.Equal(t, 2, m.Components.Responses.Len())
	assert.Equal(t, 3, m.Components.Headers.Len())
	assert.Equal(t, 1, m.Components.Examples.Len())
	assert.Equal(t, 1, m.Components.Links.Len())
	assert.Equal(t, 1, m.Components.Callbacks.Len())

	doc, _ = pruneSchema(doc)
	model, _ = doc.BuildV3Model()
	m = &model.Model

	assert.Equal(t, 0, m.Components.Schemas.Len())
	assert.Equal(t, 0, m.Components.Parameters.Len())
	assert.Equal(t, 0, m.Components.RequestBodies.Len())
	assert.Equal(t, 0, m.Components.Responses.Len())
	assert.Equal(t, 0, m.Components.Headers.Len())
	assert.Equal(t, 0, m.Components.Examples.Len())
	assert.Equal(t, 0, m.Components.Links.Len())
	assert.Equal(t, 0, m.Components.Callbacks.Len())
}

const pruneComprehensiveTestFixture = `
openapi: 3.0.1

info:
  title: OpenAPI-CodeGen Test
  description: 'This is a test OpenAPI Spec'
  version: 1.0.0

servers:
- url: https://test.oapi-codegen.com/v2
- url: http://test.oapi-codegen.com/v2

paths:
  /test:
    get:
      operationId: doesNothing
      summary: does nothing
      tags: [nothing]
      responses:
        default:
          description: returns nothing
          content:
            application/json:
              schema:
                type: object
components:
  schemas:
    Object1:
      type: object
      properties:
        object:
          $ref: "#/components/schemas/Object2"
    Object2:
      type: object
      properties:
        object:
          $ref: "#/components/schemas/Object3"
    Object3:
      type: object
      properties:
        object:
          $ref: "#/components/schemas/Object4"
    Object4:
      type: object
      properties:
        object:
          $ref: "#/components/schemas/Object5"
    Object5:
      type: object
      properties:
        object:
          $ref: "#/components/schemas/Object6"
    Object6:
      type: object
    Pet:
      type: object
      required:
        - id
        - name
      properties:
        id:
          type: integer
          format: int64
        name:
          type: string
        tag:
          type: string
    Error:
      required:
        - code
        - message
      properties:
        code:
          type: integer
          format: int32
          description: Error code
        message:
          type: string
          description: Error message
  parameters:
    offsetParam:
      name: offset
      in: query
      description: Number of items to skip before returning the results.
      required: false
      schema:
        type: integer
        format: int32
        minimum: 0
        default: 0
  securitySchemes:
    BasicAuth:
      type: http
      scheme: basic
    BearerAuth:
      type: http
      scheme: bearer
  requestBodies:
    PetBody:
      description: A JSON object containing pet information
      required: true
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Pet'
  responses:
    NotFound:
      description: The specified resource was not found
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Error'
    Unauthorized:
      description: Unauthorized
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Error'
  headers:
    X-RateLimit-Limit:
      schema:
        type: integer
      description: Request limit per hour.
    X-RateLimit-Remaining:
      schema:
        type: integer
      description: The number of requests left for the time window.
    X-RateLimit-Reset:
      schema:
        type: string
        format: date-time
      description: The UTC date/time at which the current rate limit window resets
  examples:
    objectExample:
      value:
        id: 1
        name: new object
      summary: A sample object
  links:
    GetUserByUserId:
      description: >
        The id value returned in the response can be used as
        the userId parameter in GET /users/{userId}.
      operationId: getUser
      parameters:
        userId: '$response.body#/id'
  callbacks:
    MyCallback:
      '{$request.body#/callbackUrl}':
        post:
          requestBody:
            required: true
            content:
              application/json:
                schema:
                  type: object
                  properties:
                    message:
                      type: string
                      example: Some event happened
                  required:
                    - message
          responses:
            '200':
              description: Your server returns this code if it accepts the callback
`

const pruneSpecTestFixture = `
openapi: 3.0.1

info:
  title: OpenAPI-CodeGen Test
  description: 'This is a test OpenAPI Spec'
  version: 1.0.0

servers:
- url: https://test.oapi-codegen.com/v2
- url: http://test.oapi-codegen.com/v2

paths:
  /cat:
    get:
      tags:
        - cat
      summary: Get cat status
      operationId: getCatStatus
      responses:
        200:
          description: Success
          content:
            application/json:
              schema:
                oneOf:
                  - $ref: '#/components/schemas/CatAlive'
                  - $ref: '#/components/schemas/CatDead'
            application/xml:
              schema:
                anyOf:
                  - $ref: '#/components/schemas/CatAlive'
                  - $ref: '#/components/schemas/CatDead'
            application/yaml:
              schema:
                allOf:
                  - $ref: '#/components/schemas/CatAlive'
                  - $ref: '#/components/schemas/CatDead'
        default:
          description: Error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'

  /dog:
    get:
      tags:
        - dog
      summary: Get dog status
      operationId: getDogStatus
      responses:
        200:
          description: Success
          content:
            application/json:
              schema:
                oneOf:
                  - $ref: '#/components/schemas/DogAlive'
                  - $ref: '#/components/schemas/DogDead'
            application/xml:
              schema:
                anyOf:
                  - $ref: '#/components/schemas/DogAlive'
                  - $ref: '#/components/schemas/DogDead'
            application/yaml:
              schema:
                allOf:
                  - $ref: '#/components/schemas/DogAlive'
                  - $ref: '#/components/schemas/DogDead'
        default:
          description: Error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'

components:
  schemas:

    Error:
      properties:
        code:
          type: integer
          format: int32
        message:
          type: string

    CatAlive:
      properties:
        name:
          type: string
        alive_since:
          type: string
          format: date-time

    CatDead:
      properties:
        name:
          type: string
        dead_since:
          type: string
          format: date-time
        cause:
          type: string
          enum: [car, dog, oldage]

    DogAlive:
      properties:
        name:
          type: string
        alive_since:
          type: string
          format: date-time

    DogDead:
      properties:
        name:
          type: string
        dead_since:
          type: string
          format: date-time
        cause:
          type: string
          enum: [car, cat, oldage]

`

func TestPruneParameterSchemaRefs(t *testing.T) {
	// Test that schemas referenced by component parameters are not pruned
	doc, err := LoadDocumentFromContents([]byte(pruneParameterSchemaTestFixture))
	assert.NoError(t, err)

	model, _ := doc.BuildV3Model()
	m := &model.Model

	// Before pruning: should have all schemas
	assert.Equal(t, 3, m.Components.Schemas.Len(), "Should have 3 schemas before pruning")
	assert.Equal(t, 2, m.Components.Parameters.Len(), "Should have 2 parameters before pruning")

	// Prune the schema
	doc, err = pruneSchema(doc)
	assert.NoError(t, err)

	model, _ = doc.BuildV3Model()
	m = &model.Model

	// After pruning: schemas referenced by parameters should be preserved
	assert.Equal(t, 2, m.Components.Schemas.Len(), "Should have 2 schemas after pruning (DateProp and FormatProp)")
	assert.Equal(t, 2, m.Components.Parameters.Len(), "Should have 2 parameters after pruning")

	// Verify the specific schemas are preserved
	assert.NotNil(t, m.Components.Schemas.GetOrZero("DateProp"), "DateProp schema should be preserved")
	assert.NotNil(t, m.Components.Schemas.GetOrZero("FormatProp"), "FormatProp schema should be preserved")
	assert.Nil(t, m.Components.Schemas.GetOrZero("UnusedSchema"), "UnusedSchema should be pruned")
}

const pruneParameterSchemaTestFixture = `
openapi: 3.0.0
info:
  title: Parameter Schema Test
  version: 1.0.0
paths:
  /items:
    get:
      operationId: listItems
      parameters:
        - $ref: "#/components/parameters/DateStart"
        - $ref: "#/components/parameters/Format"
      responses:
        '200':
          description: Success
          content:
            application/json:
              schema:
                type: object
components:
  parameters:
    DateStart:
      name: f.datum.start
      in: query
      schema:
        $ref: "#/components/schemas/DateProp"
      example: "2021-06-13"
    Format:
      name: format
      in: query
      schema:
        $ref: "#/components/schemas/FormatProp"
  schemas:
    DateProp:
      type: string
      format: date
    FormatProp:
      type: string
      enum: [json, xml]
    UnusedSchema:
      type: string
      description: This schema is not referenced and should be pruned
`

func TestPruneInlineParameterSchemaRefs(t *testing.T) {
	t.Run("schemas referenced in inline path parameters should not be pruned", func(t *testing.T) {
		contents, err := os.ReadFile("testdata/prune-param-schema-refs.yml")
		assert.NoError(t, err)

		doc, err := LoadDocumentFromContents(contents)
		assert.NoError(t, err)

		model, err := doc.BuildV3Model()
		assert.NoError(t, err)

		// Before filtering: should have 4 schemas (UserId, User, ItemId, Item)
		assert.Equal(t, 4, model.Model.Components.Schemas.Len())

		// Filter to only include "users" tag
		cfg := FilterConfig{
			Include: FilterParamsConfig{
				Tags: []string{"users"},
			},
		}

		filterOperations(&model.Model, cfg)

		_, doc2, _, err := doc.RenderAndReload()
		assert.NoError(t, err)

		// Prune unused schemas
		doc3, err := pruneSchema(doc2)
		assert.NoError(t, err)

		model3, err := doc3.BuildV3Model()
		assert.NoError(t, err)

		// After pruning: should have 2 schemas (UserId, User)
		// UserId should NOT be pruned even though it's only referenced in path parameter
		assert.Equal(t, 2, model3.Model.Components.Schemas.Len())

		// Verify UserId and User exist
		_, hasUserId := model3.Model.Components.Schemas.Get("UserId")
		assert.True(t, hasUserId, "UserId should not be pruned - it's referenced in path parameter")

		_, hasUser := model3.Model.Components.Schemas.Get("User")
		assert.True(t, hasUser, "User should not be pruned - it's referenced in response")

		// Verify ItemId and Item were pruned
		_, hasItemId := model3.Model.Components.Schemas.Get("ItemId")
		assert.False(t, hasItemId, "ItemId should be pruned - items tag was filtered out")

		_, hasItem := model3.Model.Components.Schemas.Get("Item")
		assert.False(t, hasItem, "Item should be pruned - items tag was filtered out")
	})
}
