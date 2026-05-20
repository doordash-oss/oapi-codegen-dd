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

	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
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

	t.Run("nested allOf inside a no-type wrapper preserves properties", func(t *testing.T) {
		// When an allOf entry is itself an allOf wrapper without an
		// explicit `type`, the merge has to flatten the nested allOf
		// before checking types: otherwise the type-presence
		// short-circuit drops the wrapper entirely and the composed
		// schema ends up with only the second branch's properties.
		contents, err := os.ReadFile("testdata/merge-nested-allof-no-type-wrapper.yml")
		require.NoError(t, err)

		opts := Configuration{
			PackageName: "testpkg",
			Output: &Output{
				UseSingleFile: true,
			},
		}

		code, err := Generate(contents, opts)
		require.NoError(t, err)
		combined := code.GetCombined()

		assert.Contains(t, combined, "type ContactDetails struct",
			"ContactDetails should be a struct")
		// Properties from the nested allOf wrapper must survive.
		assert.Regexp(t, `(?m)ContactID\s+\*?string`, combined,
			"ContactID must come through from the nested allOf wrapper")
		assert.Regexp(t, `(?m)Note\s+\*?string`, combined,
			"Note must come through from the nested allOf wrapper")
		// And the outer branch's own properties stay.
		assert.Regexp(t, `(?m)Tags\s+\*?\[\]string`, combined,
			"Tags must stay alongside the inherited fields")
	})

	t.Run("allOf with colliding array items keeps the later branch", func(t *testing.T) {
		// Regression: when two allOf branches both declare `items`,
		// mergeItems used to pick the first branch and silently drop
		// the second, losing any anyOf/oneOf union inside it. allOf
		// composition refines an earlier declaration, so the later
		// branch should win - same rule as property collisions.
		contents := []byte(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Test
paths:
  /test:
    post:
      operationId: testEndpoint
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CombinedError'
      responses:
        '200':
          description: ok
components:
  schemas:
    BaseError:
      type: object
      properties:
        issues:
          type: array
          items:
            type: object
            properties:
              field:
                type: string
              issue:
                type: string
    SpecificError:
      properties:
        issues:
          type: array
          items:
            anyOf:
              - title: ERROR_TYPE_A
                properties:
                  issue:
                    type: string
                    enum:
                      - ERROR_A
              - title: ERROR_TYPE_B
                properties:
                  issue:
                    type: string
                    enum:
                      - ERROR_B
    CombinedError:
      allOf:
        - $ref: '#/components/schemas/BaseError'
        - $ref: '#/components/schemas/SpecificError'
`)
		opts := Configuration{
			PackageName: "testpkg",
			Output:      &Output{UseSingleFile: true},
		}

		code, err := Generate(contents, opts)
		require.NoError(t, err)
		combined := code.GetCombined()

		// The anyOf branches from SpecificError must survive the
		// merge - the previous bug collapsed CombinedError items to
		// just BaseError's {field, issue} object and dropped the
		// ERROR_A/ERROR_B enums entirely.
		assert.Contains(t, combined, "ERRORA",
			"ERROR_A enum from SpecificError's anyOf must survive the merge")
		assert.Contains(t, combined, "ERRORB",
			"ERROR_B enum from SpecificError's anyOf must survive the merge")
		assert.Contains(t, combined, "CombinedError_Issues_AnyOf",
			"the anyOf union type on items must be generated for CombinedError")
	})

	t.Run("overlapping property keeps typed declaration", func(t *testing.T) {
		// When two allOf branches declare the same property and only
		// one of them gives it a type (the other only attaches an
		// annotation such as `example`), the typed declaration must
		// survive the merge. Previously the second branch silently
		// overwrote the first, turning a `status: string` into an
		// empty struct.
		contents, err := os.ReadFile("testdata/merge-overlapping-property-typed-wins.yml")
		require.NoError(t, err)

		opts := Configuration{
			PackageName: "testpkg",
			Output: &Output{
				UseSingleFile: true,
			},
		}

		code, err := Generate(contents, opts)
		require.NoError(t, err)
		combined := code.GetCombined()

		assert.Contains(t, combined, "type InvoiceCancellation struct",
			"InvoiceCancellation should be a struct")
		assert.Regexp(t, `(?m)Status\s+\*?string`, combined,
			"Status should keep the string type from the referenced Invoice schema, not collapse to an empty struct from the annotation-only override")
		assert.NotRegexp(t, `(?m)Status\s+\*?struct\{\}`, combined,
			"Status must not be generated as struct{}; that would mean the annotation-only override dropped the typed declaration")
	})

	t.Run("allOf with sibling properties preserves all fields", func(t *testing.T) {
		contents, err := os.ReadFile("testdata/allof-with-sibling-properties.yml")
		require.NoError(t, err)

		opts := Configuration{
			PackageName: "testpkg",
			Output: &Output{
				UseSingleFile: true,
			},
		}

		code, err := Generate(contents, opts)
		require.NoError(t, err)
		combined := code.GetCombined()

		// --- Case 1: allOf with sibling properties ---
		// ChatPrompt should be a struct with both its own 'prompt' field AND BasePrompt fields
		assert.Contains(t, combined, "type ChatPrompt struct")
		assert.NotContains(t, combined, "type ChatPrompt = BasePrompt",
			"ChatPrompt should be a struct, not a type alias")

		// TextPrompt should also be a struct with both its own 'prompt' field AND BasePrompt fields
		assert.Contains(t, combined, "type TextPrompt struct")
		assert.NotContains(t, combined, "type TextPrompt = BasePrompt",
			"TextPrompt should be a struct, not a type alias")

		// The oneOf inline structs should also include the sibling properties from
		// ChatPrompt/TextPrompt (the 'prompt' field) alongside the BasePrompt fields.
		// This verifies that transitive allOf merging preserves sibling properties.
		assert.Contains(t, combined, "type Prompt_OneOf_0 struct",
			"Prompt_OneOf_0 should be generated")
		assert.Contains(t, combined, "type Prompt_OneOf_1 struct",
			"Prompt_OneOf_1 should be generated")

		// Prompt_OneOf_0 (chat variant) should have Prompt as []string
		assert.Contains(t, combined, `Prompt  []string`,
			"Prompt_OneOf_0 should have Prompt []string from ChatPrompt")
		// Prompt_OneOf_1 (text variant) should have Prompt as string
		assert.Contains(t, combined, `Prompt  string`,
			"Prompt_OneOf_1 should have Prompt string from TextPrompt")

		// --- Case 2: oneOf with sibling properties ---
		// Event has its own 'timestamp' property alongside a oneOf.
		// The timestamp must not be silently dropped.
		assert.Contains(t, combined, "type Event struct",
			"Event should be a struct")
		assert.Contains(t, combined, `Timestamp`,
			"Event should have its own Timestamp field")

		// --- Case 3: allOf with format annotation on parent ---
		// Customer has format: "customer_v1" alongside allOf + properties.
		// The format must not cause a merge conflict during transitive flattening.
		assert.Contains(t, combined, "type Customer struct",
			"Customer should be a struct")
		assert.Contains(t, combined, `Name`,
			"Customer should have its own Name field")
		assert.Contains(t, combined, `Email`,
			"Customer should have Email from CustomerBase via allOf")

		// --- Case 4: allOf only, no sibling properties (regression guard) ---
		// SimpleAlias has allOf with a single $ref and no own properties.
		// It should still collapse to an alias, not be broken by the fix.
		assert.Contains(t, combined, "type SimpleAlias = BasePrompt",
			"SimpleAlias should remain a type alias when there are no sibling properties")
		assert.NotContains(t, combined, "type SimpleAlias struct",
			"SimpleAlias should not become a struct")

		// --- Case 5: properties only, no allOf (regression guard) ---
		assert.Contains(t, combined, "type PlainObject struct",
			"PlainObject should be a normal struct")
		assert.Contains(t, combined, `ID`,
			"PlainObject should have its ID field")

		// --- Case 6: anyOf with sibling properties ---
		// AppConfig has its own 'appName' alongside an anyOf.
		assert.Contains(t, combined, "type AppConfig struct",
			"AppConfig should be a struct")
		assert.Contains(t, combined, `AppName`,
			"AppConfig should have its own AppName field")
	})
}

func TestSingleElementUnionOptimization(t *testing.T) {
	t.Run("single anyOf with ref should use type directly", func(t *testing.T) {
		doc := loadUnionDocument(t)

		// Get the Order schema which has client: anyOf: [Identity]
		orderSchema := doc.Components.Schemas.Value("Order")
		require.NotNil(t, orderSchema)

		res, err := GenerateGoSchema(orderSchema, ParseOptions{typeTracker: newTypeTracker()}.WithPath([]string{"Order"}))
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

		res, err := GenerateGoSchema(verificationSchema, ParseOptions{typeTracker: newTypeTracker()}.WithPath([]string{"Verification"}))
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

		res, err := GenerateGoSchema(mediaType.Schema, ParseOptions{typeTracker: newTypeTracker()}.WithPath([]string{"Response"}))
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

		res, err := GenerateGoSchema(mediaType.Schema, ParseOptions{typeTracker: newTypeTracker()}.WithPath([]string{"Response"}))
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

		res, err := GenerateGoSchema(mediaType.Schema, ParseOptions{typeTracker: newTypeTracker()}.WithPath([]string{"Response"}))
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

	t.Run("allOf with single element containing enum should generate enum type", func(t *testing.T) {
		// This test verifies that allOf with a single element that has an enum
		// generates the correct enum type, not struct{}.
		// See: https://github.com/box/box-openapi/issues/XXX
		contents := []byte(`
openapi: 3.0.0
info:
  title: Test
  version: 1.0.0
paths:
  /test:
    get:
      operationId: getTest
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/TestObject'
components:
  schemas:
    TestObject:
      type: object
      properties:
        sync_state:
          allOf:
            - description: Test description
              enum:
                - synced
                - not_synced
              example: synced
              nullable: false
              type: string
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

		// Should NOT generate struct{} for sync_state
		assert.NotContains(t, combined, "SyncState *struct {")

		// Should generate a proper enum type
		assert.Contains(t, combined, "SyncState")
		assert.Contains(t, combined, "synced")
		assert.Contains(t, combined, "not_synced")
	})

	t.Run("allOf with primitive type and nullable should generate primitive type", func(t *testing.T) {
		// This test verifies that allOf with a primitive type element and a nullable-only element
		// generates the correct primitive type, not struct{}.
		contents := []byte(`
openapi: 3.0.0
info:
  title: Test
  version: 1.0.0
paths:
  /test:
    get:
      operationId: getTest
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/TestObject'
components:
  schemas:
    TestObject:
      type: object
      properties:
        tags:
          allOf:
            - description: Tags
              items:
                type: string
              type: array
            - nullable: false
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

		// Should NOT generate struct{} for tags
		assert.NotContains(t, combined, "Tags *struct {")

		// Should generate a proper array type
		assert.Contains(t, combined, "Tags")
		assert.Contains(t, combined, "[]string")
	})
}

func TestCircularAllOfWithDiscriminatedUnion(t *testing.T) {
	// This tests the fix for circular allOf patterns where:
	// - CounterParty is a discriminated union (oneOf with discriminator)
	// - VendorDetails extends CounterParty via allOf
	// - CounterParty's oneOf includes VendorDetails
	// Without the fix, this would create an infinite loop during JSON unmarshaling
	// because VendorDetails would embed CounterParty which contains VendorDetails.
	contents, err := os.ReadFile("testdata/circular-allof-discriminator.yml")
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

	// VendorDetails should NOT embed CounterParty (which would cause circular reference)
	assert.NotContains(t, combined, "type VendorDetails struct {\n\tCounterParty")

	// VendorDetails should have its own properties from the inline schema
	assert.Contains(t, combined, "type VendorDetails struct")
	assert.Contains(t, combined, "PaymentInstrumentID")

	// BookTransferDetails should also NOT embed CounterParty
	assert.NotContains(t, combined, "type BookTransferDetails struct {\n\tCounterParty")
	assert.Contains(t, combined, "type BookTransferDetails struct")
	assert.Contains(t, combined, "SourceAccountID")
}

func TestIsDiscriminatedUnionWithChild(t *testing.T) {
	loadDoc := func(t *testing.T, contents []byte) *libopenapi.DocumentModel[v3.Document] {
		t.Helper()
		srcDoc, err := LoadDocumentFromContents(contents)
		require.NoError(t, err)
		v3Model, err := srcDoc.BuildV3Model()
		require.NoError(t, err)
		return v3Model
	}

	t.Run("returns true when oneOf contains child ref", func(t *testing.T) {
		contents := []byte(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Test
paths: {}
components:
  schemas:
    Parent:
      discriminator:
        propertyName: type
        mapping:
          CHILD: '#/components/schemas/Child'
      oneOf:
        - $ref: '#/components/schemas/Child'
    Child:
      type: object
      properties:
        name:
          type: string
`)
		doc := loadDoc(t, contents)
		parentSchema := doc.Model.Components.Schemas.GetOrZero("Parent").Schema()
		result := isDiscriminatedUnionWithChild(parentSchema, "#/components/schemas/Child")
		assert.True(t, result)
	})

	t.Run("returns false when oneOf does not contain child ref", func(t *testing.T) {
		contents := []byte(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Test
paths: {}
components:
  schemas:
    Parent:
      discriminator:
        propertyName: type
        mapping:
          OTHER: '#/components/schemas/Other'
      oneOf:
        - $ref: '#/components/schemas/Other'
    Other:
      type: object
      properties:
        name:
          type: string
`)
		doc := loadDoc(t, contents)
		parentSchema := doc.Model.Components.Schemas.GetOrZero("Parent").Schema()
		result := isDiscriminatedUnionWithChild(parentSchema, "#/components/schemas/Child")
		assert.False(t, result)
	})

	t.Run("returns false when no discriminator", func(t *testing.T) {
		contents := []byte(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Test
paths: {}
components:
  schemas:
    Parent:
      oneOf:
        - $ref: '#/components/schemas/Child'
    Child:
      type: object
      properties:
        name:
          type: string
`)
		doc := loadDoc(t, contents)
		parentSchema := doc.Model.Components.Schemas.GetOrZero("Parent").Schema()
		result := isDiscriminatedUnionWithChild(parentSchema, "#/components/schemas/Child")
		assert.False(t, result)
	})

	t.Run("returns false for nil schema", func(t *testing.T) {
		result := isDiscriminatedUnionWithChild(nil, "#/components/schemas/Child")
		assert.False(t, result)
	})

	t.Run("returns false for inheritance base with discriminator but no oneOf", func(t *testing.T) {
		contents := []byte(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Test
paths: {}
components:
  schemas:
    Parent:
      type: object
      properties:
        kind:
          type: string
      discriminator:
        propertyName: kind
        mapping:
          CHILD: '#/components/schemas/Child'
    Child:
      allOf:
        - $ref: '#/components/schemas/Parent'
        - type: object
          properties:
            extra:
              type: string
`)
		doc := loadDoc(t, contents)
		parentSchema := doc.Model.Components.Schemas.GetOrZero("Parent").Schema()
		result := isDiscriminatedUnionWithChild(parentSchema, "#/components/schemas/Child")
		assert.False(t, result)
	})

	t.Run("returns true for anyOf with child ref", func(t *testing.T) {
		contents := []byte(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Test
paths: {}
components:
  schemas:
    Parent:
      discriminator:
        propertyName: type
      anyOf:
        - $ref: '#/components/schemas/Child'
    Child:
      type: object
      properties:
        name:
          type: string
`)
		doc := loadDoc(t, contents)
		parentSchema := doc.Model.Components.Schemas.GetOrZero("Parent").Schema()
		result := isDiscriminatedUnionWithChild(parentSchema, "#/components/schemas/Child")
		assert.True(t, result)
	})
}

func TestIfThenElse(t *testing.T) {
	contents, err := os.ReadFile("testdata/if-then-else.yml")
	require.NoError(t, err)

	opts := Configuration{
		PackageName: "testpkg",
		Output: &Output{
			UseSingleFile: true,
		},
	}

	code, err := Generate(contents, opts)
	require.NoError(t, err)
	combined := code.GetCombined()

	t.Run("basic if/then/else generates union with named variants", func(t *testing.T) {
		// BasicIfThenElse should have its own properties plus a union wrapper
		assert.Contains(t, combined, "type BasicIfThenElse struct")
		assert.Contains(t, combined, "Kind")

		// Union wrapper type with named variants (_Then/_Else, not _0/_1)
		assert.Contains(t, combined, "BasicIfThenElse_IfThenElse")
		assert.Contains(t, combined, "type BasicIfThenElse_Then struct")
		assert.Contains(t, combined, "type BasicIfThenElse_Else struct")
		assert.Contains(t, combined, "Conditional[BasicIfThenElse_Then, BasicIfThenElse_Else]")

		// Properties from then branch
		assert.Contains(t, combined, "FieldA")
		assert.Contains(t, combined, "ValueA")

		// Properties from else branch
		assert.Contains(t, combined, "FieldB")
		assert.Contains(t, combined, "ValueB")
	})

	t.Run("then only flat merges", func(t *testing.T) {
		// ThenOnly should have properties from both the base schema and the then branch
		assert.Contains(t, combined, "type ThenOnly struct")
		assert.Contains(t, combined, "Enabled")

		// With a single branch, the then properties should be merged into the parent,
		// so we should see Config and Timeout as fields
		assert.Contains(t, combined, "Config")
		assert.Contains(t, combined, "Timeout")
	})

	t.Run("if/then/else inside allOf", func(t *testing.T) {
		// InsideAllOf should inherit from BaseResource and handle the conditional
		assert.Contains(t, combined, "InsideAllOf")
		assert.Contains(t, combined, "Category")
	})

	t.Run("nested if/then/else", func(t *testing.T) {
		// Nested should handle the outer if/then/else
		assert.Contains(t, combined, "Nested")
		assert.Contains(t, combined, "Level")
	})

	t.Run("validation only if/then does not break generation", func(t *testing.T) {
		// ValidationOnly should generate without errors even when then only adds required.
		// Since the then branch only adds "required" with no new properties or type,
		// it resolves as an empty/zero schema and is effectively a no-op for code
		// generation. The parent properties (mode, requiredInStrict) are still present
		// but the then branch contributes no structural types.
		assert.Contains(t, combined, "ValidationOnly")
	})
}
