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
		contents, err := os.ReadFile("testdata/prune-cat-dog.yml")
		assert.NoError(t, err)

		doc, err := LoadDocumentFromContents(contents)
		assert.NoError(t, err)

		model, _ := doc.BuildV3Model()

		refs := findOperationRefs(&model.Model)
		assert.Len(t, refs, 5)
	})

	t.Run("only cat", func(t *testing.T) {
		contents, err := os.ReadFile("testdata/prune-cat-dog.yml")
		assert.NoError(t, err)

		doc, err := LoadDocumentFromContents(contents)
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
		contents, err := os.ReadFile("testdata/prune-cat-dog.yml")
		assert.NoError(t, err)

		doc, err := LoadDocumentFromContents(contents)
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
	contents, err := os.ReadFile("testdata/prune-cat-dog.yml")
	assert.NoError(t, err)

	doc, err := LoadDocumentFromContents(contents)
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

	doc, err = pruneSchema(doc2, oapiCodegenExtensions)
	assert.Nil(t, err)
	if err != nil {
		t.Fatal(err)
	}
	model, _ = doc.BuildV3Model()

	assert.Equal(t, 3, model.Model.Components.Schemas.Len())
}

func TestFilterOnlyDog(t *testing.T) {
	contents, err := os.ReadFile("testdata/prune-cat-dog.yml")
	assert.NoError(t, err)

	doc, err := LoadDocumentFromContents(contents)
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

	doc3, _ := pruneSchema(doc2, oapiCodegenExtensions)
	assert.Nil(t, err)
	if err != nil {
		t.Fatal(err)
	}
	m3, _ := doc3.BuildV3Model()

	assert.Equal(t, 3, m3.Model.Components.Schemas.Len())
}

func TestPruningUnusedComponents(t *testing.T) {
	contents, err := os.ReadFile("testdata/prune-all-components.yml")
	assert.NoError(t, err)

	doc, err := LoadDocumentFromContents(contents)
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

	doc, _ = pruneSchema(doc, oapiCodegenExtensions)
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

func TestPruneParameterSchemaRefs(t *testing.T) {
	// Test that schemas referenced by component parameters are not pruned
	contents, err := os.ReadFile("testdata/prune-component-params.yml")
	assert.NoError(t, err)

	doc, err := LoadDocumentFromContents(contents)
	assert.NoError(t, err)

	model, _ := doc.BuildV3Model()
	m := &model.Model

	// Before pruning: should have all schemas
	assert.Equal(t, 3, m.Components.Schemas.Len(), "Should have 3 schemas before pruning")
	assert.Equal(t, 2, m.Components.Parameters.Len(), "Should have 2 parameters before pruning")

	// Prune the schema
	doc, err = pruneSchema(doc, oapiCodegenExtensions)
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
		doc3, err := pruneSchema(doc2, oapiCodegenExtensions)
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

func TestPruneComponentSchemaRefs(t *testing.T) {
	t.Run("component schemas that are refs to other schemas should preserve the target", func(t *testing.T) {
		contents, err := os.ReadFile("testdata/prune-component-schema-refs.yml")
		assert.NoError(t, err)

		doc, err := LoadDocumentFromContents(contents)
		assert.NoError(t, err)

		model, err := doc.BuildV3Model()
		assert.NoError(t, err)

		// Before pruning: should have 6 schemas
		// AuthSpecification, SourceAuthSpecification, DestinationAuthSpecification,
		// SourceDefinitionSpecificationRead, DestinationDefinitionSpecificationRead, UnusedSchema
		assert.Equal(t, 6, model.Model.Components.Schemas.Len())

		// Prune unused schemas
		doc2, err := pruneSchema(doc, oapiCodegenExtensions)
		assert.NoError(t, err)

		model2, err := doc2.BuildV3Model()
		assert.NoError(t, err)

		// After pruning: should have 5 schemas (all except UnusedSchema)
		// AuthSpecification should NOT be pruned even though it's only referenced
		// by other component schemas (SourceAuthSpecification, DestinationAuthSpecification)
		assert.Equal(t, 5, model2.Model.Components.Schemas.Len())

		// Verify all the necessary schemas exist
		_, hasAuthSpec := model2.Model.Components.Schemas.Get("AuthSpecification")
		assert.True(t, hasAuthSpec, "AuthSpecification should not be pruned - it's referenced by component schemas")

		_, hasSourceAuthSpec := model2.Model.Components.Schemas.Get("SourceAuthSpecification")
		assert.True(t, hasSourceAuthSpec, "SourceAuthSpecification should not be pruned")

		_, hasDestAuthSpec := model2.Model.Components.Schemas.Get("DestinationAuthSpecification")
		assert.True(t, hasDestAuthSpec, "DestinationAuthSpecification should not be pruned")

		_, hasSourceDefSpec := model2.Model.Components.Schemas.Get("SourceDefinitionSpecificationRead")
		assert.True(t, hasSourceDefSpec, "SourceDefinitionSpecificationRead should not be pruned")

		_, hasDestDefSpec := model2.Model.Components.Schemas.Get("DestinationDefinitionSpecificationRead")
		assert.True(t, hasDestDefSpec, "DestinationDefinitionSpecificationRead should not be pruned")

		// Verify UnusedSchema was pruned
		_, hasUnused := model2.Model.Components.Schemas.Get("UnusedSchema")
		assert.False(t, hasUnused, "UnusedSchema should be pruned - it's not referenced anywhere")
	})
}

func TestPruneComponentRequestBodyRefs(t *testing.T) {
	t.Run("component request bodies that are refs to other request bodies should preserve the target", func(t *testing.T) {
		contents, err := os.ReadFile("testdata/prune-component-requestbody-refs.yml")
		assert.NoError(t, err)

		doc, err := LoadDocumentFromContents(contents)
		assert.NoError(t, err)

		model, err := doc.BuildV3Model()
		assert.NoError(t, err)

		// Before pruning: should have 4 request bodies and 3 schemas
		assert.Equal(t, 4, model.Model.Components.RequestBodies.Len())
		assert.Equal(t, 3, model.Model.Components.Schemas.Len())

		// Prune unused components
		doc2, err := pruneSchema(doc, oapiCodegenExtensions)
		assert.NoError(t, err)

		model2, err := doc2.BuildV3Model()
		assert.NoError(t, err)

		// After pruning: should have 3 request bodies (all except UnusedRequest)
		// CursorRequest should NOT be pruned even though it's only referenced
		// by other component request bodies (AuditEventsRequest, ItemUsagesRequest)
		assert.Equal(t, 3, model2.Model.Components.RequestBodies.Len())

		// After pruning: should have 2 schemas (Cursor, ResetCursor - UnusedSchema should be pruned)
		assert.Equal(t, 2, model2.Model.Components.Schemas.Len())

		// Verify all the necessary request bodies exist
		_, hasCursorReq := model2.Model.Components.RequestBodies.Get("CursorRequest")
		assert.True(t, hasCursorReq, "CursorRequest should not be pruned - it's referenced by component request bodies")

		_, hasAuditReq := model2.Model.Components.RequestBodies.Get("AuditEventsRequest")
		assert.True(t, hasAuditReq, "AuditEventsRequest should not be pruned")

		_, hasItemReq := model2.Model.Components.RequestBodies.Get("ItemUsagesRequest")
		assert.True(t, hasItemReq, "ItemUsagesRequest should not be pruned")

		// Verify UnusedRequest was pruned
		_, hasUnusedReq := model2.Model.Components.RequestBodies.Get("UnusedRequest")
		assert.False(t, hasUnusedReq, "UnusedRequest should be pruned - it's not referenced anywhere")

		// Verify schemas
		_, hasCursor := model2.Model.Components.Schemas.Get("Cursor")
		assert.True(t, hasCursor, "Cursor schema should not be pruned")

		_, hasResetCursor := model2.Model.Components.Schemas.Get("ResetCursor")
		assert.True(t, hasResetCursor, "ResetCursor schema should not be pruned")

		_, hasUnusedSchema := model2.Model.Components.Schemas.Get("UnusedSchema")
		assert.False(t, hasUnusedSchema, "UnusedSchema should be pruned - it's not referenced anywhere")
	})
}

func TestPruneDefaultResponseHeaders(t *testing.T) {
	t.Run("headers referenced in default responses should not be pruned", func(t *testing.T) {
		contents, err := os.ReadFile("testdata/prune-default-response-headers.yml")
		assert.NoError(t, err)

		doc, err := LoadDocumentFromContents(contents)
		assert.NoError(t, err)

		model, err := doc.BuildV3Model()
		assert.NoError(t, err)

		// Before pruning: should have 3 headers and 3 schemas
		assert.Equal(t, 3, model.Model.Components.Headers.Len())
		assert.Equal(t, 3, model.Model.Components.Schemas.Len())

		// Prune unused components
		doc2, err := pruneSchema(doc, oapiCodegenExtensions)
		assert.NoError(t, err)

		model2, err := doc2.BuildV3Model()
		assert.NoError(t, err)

		// After pruning: should have 2 headers (ErrorCode, ErrorMessage - UnusedHeader should be pruned)
		assert.Equal(t, 2, model2.Model.Components.Headers.Len())

		// After pruning: should have 2 schemas (SuccessResponse, ErrorResponse - UnusedSchema should be pruned)
		assert.Equal(t, 2, model2.Model.Components.Schemas.Len())

		// Verify ErrorCode and ErrorMessage headers exist
		_, hasErrorCode := model2.Model.Components.Headers.Get("ErrorCode")
		assert.True(t, hasErrorCode, "ErrorCode header should not be pruned - it's referenced in default response")

		_, hasErrorMessage := model2.Model.Components.Headers.Get("ErrorMessage")
		assert.True(t, hasErrorMessage, "ErrorMessage header should not be pruned - it's referenced in default response")

		// Verify UnusedHeader was pruned
		_, hasUnusedHeader := model2.Model.Components.Headers.Get("UnusedHeader")
		assert.False(t, hasUnusedHeader, "UnusedHeader should be pruned - it's not referenced anywhere")

		// Verify schemas
		_, hasSuccess := model2.Model.Components.Schemas.Get("SuccessResponse")
		assert.True(t, hasSuccess, "SuccessResponse schema should not be pruned")

		_, hasError := model2.Model.Components.Schemas.Get("ErrorResponse")
		assert.True(t, hasError, "ErrorResponse schema should not be pruned")

		_, hasUnusedSchema := model2.Model.Components.Schemas.Get("UnusedSchema")
		assert.False(t, hasUnusedSchema, "UnusedSchema should be pruned - it's not referenced anywhere")
	})
}

func TestPruneExamples(t *testing.T) {
	t.Run("examples removed during pruning", func(t *testing.T) {
		contents, err := os.ReadFile("testdata/with-examples.yml")
		assert.NoError(t, err)

		doc, err := LoadDocumentFromContents(contents)
		assert.NoError(t, err)

		// Prune the document
		prunedDoc, err := pruneSchema(doc, oapiCodegenExtensions)
		assert.NoError(t, err)

		model, err := prunedDoc.BuildV3Model()
		assert.NoError(t, err)

		// components/examples should be removed by pruning (they're not referenced by operations)
		// We track example references via collectExampleRefs
		assert.Equal(t, 0, model.Model.Components.Examples.Len())

		// Schema examples are also not removed
		paySessionReq := model.Model.Components.Schemas.GetOrZero("PaySessionRequest")
		schema := paySessionReq.Schema()
		assert.NotNil(t, schema.Example)

		// Property examples should be empty, but example (singular) should be kept
		fooProp := schema.Properties.GetOrZero("foo")
		fooSchema := fooProp.Schema()
		assert.Nil(t, fooSchema.Examples)
		assert.NotNil(t, fooSchema.Example)

		// Check inline operation request body examples are removed
		sessionsPath := model.Model.Paths.PathItems.GetOrZero("/sessions")
		postOp := sessionsPath.Post
		assert.NotNil(t, postOp.RequestBody)
		jsonContent := postOp.RequestBody.Content.GetOrZero("application/json")
		assert.Equal(t, 0, jsonContent.Examples.Len())

		// Parameter examples should be empty, but example (singular) should be kept
		param := model.Model.Components.Parameters.GetOrZero("Idempotency-Key")
		assert.Equal(t, 0, param.Examples.Len())
		assert.NotNil(t, param.Example)

		// Header examples should be empty, but example (singular) should be kept
		header := model.Model.Components.Headers.GetOrZero("Idempotency-Key")
		assert.Equal(t, 0, header.Examples.Len())
		assert.NotNil(t, header.Example)
	})

	t.Run("webhooks removed during pruning", func(t *testing.T) {
		contents, err := os.ReadFile("testdata/webhooks-with-examples.yml")
		assert.NoError(t, err)

		doc, err := LoadDocumentFromContents(contents)
		assert.NoError(t, err)

		// Prune the document
		prunedDoc, err := pruneSchema(doc, oapiCodegenExtensions)
		assert.NoError(t, err)

		model, err := prunedDoc.BuildV3Model()
		assert.NoError(t, err)

		// components/examples should be removed because they're only referenced by webhooks
		// and webhooks are removed during cleanup
		assert.Equal(t, 0, model.Model.Components.Examples.Len())

		// webhooks should be removed
		assert.Nil(t, model.Model.Webhooks)
	})
}
