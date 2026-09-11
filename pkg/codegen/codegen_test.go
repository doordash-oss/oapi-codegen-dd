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
	"bytes"
	"embed"
	"go/format"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/*
var testdataFS embed.FS

func readTestdata(t *testing.T, name string) string {
	t.Helper()
	data, err := testdataFS.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("failed to read testdata/%s: %v", name, err)
	}
	return string(data)
}

// Keep these for backward compatibility with other test files
//
//go:embed testdata/test_spec.yml
var testDocument string

func TestExampleOpenAPICodeGeneration(t *testing.T) {
	// Input vars for code generation:
	packageName := "testswagger"
	cfg := Configuration{
		PackageName: packageName,
		Output: &Output{
			UseSingleFile: true,
		},
	}

	// Run our code generation:
	codes, err := Generate([]byte(readTestdata(t, "test_spec.yml")), cfg)
	require.NoError(t, err)

	code := codes.GetCombined()
	assert.NotEmpty(t, code)

	// Check that we have a package:
	assert.Contains(t, code, "package testswagger")

	assert.Contains(t, code, "Top *int `json:\"$top,omitempty\"`")
	assert.Contains(t, code, "DeadSince *time.Time    `json:\"dead_since,omitempty\" tag1:\"value1\" tag2:\"value2\"`")
	assert.Contains(t, code, "type EnumTestNumerics int")
	// With AlwaysPrefixEnumValues=false (default), enum values are unprefixed
	assert.Contains(t, code, "N2 EnumTestNumerics = 2")
	assert.Contains(t, code, "type EnumTestEnumNames int")
	assert.Contains(t, code, "Two  EnumTestEnumNames = 2")
}

func TestExtPropGoTypeSkipOptionalPointer(t *testing.T) {
	packageName := "api"
	cfg := Configuration{
		PackageName: packageName,
		Output: &Output{
			UseSingleFile: true,
		},
	}
	spec := "testdata/x-go-type-skip-optional-pointer.yml"
	docContents, err := os.ReadFile(spec)
	require.NoError(t, err)

	// Run our code generation:
	codes, err := Generate(docContents, cfg)
	assert.NoError(t, err)
	assert.NotEmpty(t, codes)

	code := codes.GetCombined()

	// Check that we have valid (formattable) code:
	_, err = format.Source([]byte(code))
	require.NoError(t, err)

	// Check that optional pointer fields are skipped if requested
	assert.Contains(t, code, "NullableFieldSkipFalse *string `json:\"nullableFieldSkipFalse,omitempty\"`")
	assert.Contains(t, code, "NullableFieldSkipTrue  string  `json:\"nullableFieldSkipTrue\"`")
	assert.Contains(t, code, "OptionalField          *string `json:\"optionalField,omitempty\"`")
	assert.Contains(t, code, "OptionalFieldSkipFalse *string `json:\"optionalFieldSkipFalse,omitempty\"`")
	assert.Contains(t, code, "OptionalFieldSkipTrue  string  `json:\"optionalFieldSkipTrue\"`")

	// Check that the extension applies on custom types as well
	assert.Contains(t, code, "CustomTypeWithSkipTrue string  `json:\"customTypeWithSkipTrue\"`")

	// Check that the extension has no effect on required fields
	assert.Contains(t, code, "RequiredField          string  `json:\"requiredField\" validate:\"required\"`")
}

func TestNumericSchemaNames(t *testing.T) {
	packageName := "api"
	cfg := Configuration{
		PackageName: packageName,
		Output: &Output{
			UseSingleFile: true,
		},
	}
	spec := "testdata/numeric-schema-names.yml"
	docContents, err := os.ReadFile(spec)
	require.NoError(t, err)

	// Run our code generation:
	codes, err := Generate(docContents, cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, codes)

	code := codes.GetCombined()

	// Check that we have valid (formattable) code:
	_, err = format.Source([]byte(code))
	require.NoError(t, err)

	// Check that numeric schema names are prefixed with "N"
	assert.Contains(t, code, "type N400 struct")
	assert.Contains(t, code, "type N401 struct")

	// Check that nested types with numeric parent schemas are also prefixed
	// Array items with properties generate TypeName_Item pattern
	assert.Contains(t, code, "type N400_Issues []N400_Issues_Item")
	assert.Contains(t, code, "type N400_Issues_Item struct")
	assert.NotContains(t, code, "type 400_Issues") // Should NOT have unprefixed version
	assert.NotContains(t, code, "[]400_Issues")    // Should NOT have unprefixed array type
	assert.NotContains(t, code, "[]struct")        // Should NOT have inline struct in array
}

func TestDuplicateLocalParameters(t *testing.T) {
	packageName := "api"
	cfg := Configuration{
		PackageName: packageName,
		Output: &Output{
			UseSingleFile: true,
		},
	}
	spec := "testdata/duplicate-local-params.yml"
	docContents, err := os.ReadFile(spec)
	require.NoError(t, err)

	// Currently, duplicate local parameters are silently ignored (first one wins)
	// This test documents the current behavior
	codes, err := Generate(docContents, cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, codes)

	code := codes.GetCombined()

	// Check that we have valid (formattable) code:
	_, err = format.Source([]byte(code))
	require.NoError(t, err)

	// The first parameter definition should be used (string, not required)
	// The duplicate (integer, required) should be silently ignored
	assert.Contains(t, code, "Filter *string")
	assert.NotContains(t, code, "Filter *int")
}

func TestGoTypeImport(t *testing.T) {
	packageName := "api"
	cfg := Configuration{
		PackageName: packageName,
		Output: &Output{
			UseSingleFile: true,
		},
	}
	spec := "testdata/x-go-type-import-pet.yml"
	docContents, err := os.ReadFile(spec)
	require.NoError(t, err)

	// Run our code generation:
	codes, err := Generate(docContents, cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, codes)
	code := codes.GetCombined()

	// Check that we have valid (formattable) code:
	_, err = format.Source([]byte(code))
	assert.NoError(t, err)

	imports := []string{
		`github.com/CavernaTechnologies/pgext`, // schemas - direct object
		`myuuid "github.com/google/uuid"`,      // schemas - object
		`github.com/lib/pq`,                    // schemas - array
		`github.com/spf13/viper`,               // responses - direct object
		`golang.org/x/text`,                    // responses - complex object
		`golang.org/x/email`,                   // requestBodies - in components
		`github.com/fatih/color`,               // parameters - query
		`github.com/go-openapi/swag`,           // parameters - path
		`github.com/jackc/pgtype`,              // direct parameters - path
		`github.com/subosito/gotenv`,           // direct request body
	}

	// Check import
	for _, imp := range imports {
		assert.Contains(t, code, imp)
	}
}

func TestBackslashEscaping(t *testing.T) {
	// Generate code
	cfg := Configuration{
		PackageName: "testbackslash",
		Output: &Output{
			UseSingleFile: true,
		},
	}

	codes, err := Generate([]byte(readTestdata(t, "backslash-escaping.yml")), cfg)
	require.NoError(t, err)
	require.NotEmpty(t, codes)

	codeStr := codes.GetCombined()

	// Verify that backslashes in enum values are properly escaped
	// The YAML has "path\\with\\backslash" which is the string path\with\backslash (1 backslash)
	// In the generated Go code, this should be "path\\with\\backslash" (2 backslashes in source)
	assert.Contains(t, codeStr, `"path\\with\\backslash"`)
	assert.Contains(t, codeStr, `"another\\value"`)

	// Verify that backslashes in discriminator mapping values are properly escaped
	// The discriminator value "bank\transfer" should become "bank\\transfer" in the case statement (2 backslashes in source)
	assert.Contains(t, codeStr, `"bank\\transfer"`)

	// Verify that the code compiles by checking it doesn't have syntax errors
	// The format.Source function will fail if there are syntax errors
	_, err = format.Source([]byte(codeStr))
	require.NoError(t, err, "Generated code should compile without syntax errors")
}

func TestBackslashEscapingJSON(t *testing.T) {
	// Generate code (JSON format)
	cfg := Configuration{
		PackageName: "testbackslash",
		Output: &Output{
			UseSingleFile: true,
		},
	}

	codes, err := Generate([]byte(readTestdata(t, "backslash-escaping.json")), cfg)
	require.NoError(t, err)
	require.NotEmpty(t, codes)

	codeStr := codes.GetCombined()

	// Verify that backslashes in enum values are properly escaped (same as YAML test)
	assert.Contains(t, codeStr, `"path\\with\\backslash"`)
	assert.Contains(t, codeStr, `"another\\value"`)

	// Verify that backslashes in discriminator mapping values are properly escaped
	assert.Contains(t, codeStr, `"bank\\transfer"`)

	// Verify that the code compiles
	_, err = format.Source([]byte(codeStr))
	require.NoError(t, err, "Generated code should compile without syntax errors")
}

func TestArrayItemPropertyNamedItem(t *testing.T) {
	// Test that when an array item has a property named "item", the array item type
	// gets a unique name (with numeric suffix) to avoid collision with the property's type.
	cfg := Configuration{
		PackageName: "testpkg",
		Output: &Output{
			UseSingleFile: true,
		},
	}

	codes, err := Generate([]byte(readTestdata(t, "array-item-property-named-item.yml")), cfg)
	require.NoError(t, err)
	require.NotEmpty(t, codes)

	code := codes.GetCombined()

	// The array item type should be named with a numeric suffix to avoid collision
	// with the "item" property's type (numeric suffixes start from 0)
	assert.Contains(t, code, "type GetTest_Response_Executions_Item struct")
	assert.Contains(t, code, "type GetTest_Response_Executions_Item0 struct")

	// The array should use the Item0 type (the array item type)
	assert.Contains(t, code, "type GetTest_Response_Executions []GetTest_Response_Executions_Item0")

	// The Item1 type should have the correct properties (id as float32, item as reference)
	assert.Contains(t, code, "ID   *float32")
	assert.Contains(t, code, "Item *GetTest_Response_Executions_Item")

	// The Item type (property type) should have string properties
	assert.Contains(t, code, "ID   *string")
	assert.Contains(t, code, "Name *string")

	// Verify that the code compiles
	_, err = format.Source([]byte(code))
	require.NoError(t, err, "Generated code should compile without syntax errors")
}

// TestOperationResponseAliasConflictWithComponentResponse tests that when an operation
// response alias would conflict with a component response name, a unique name is generated.
// When a component response references the same schema (e.g., Zone response -> Zone schema),
// the component response doesn't create a separate type. The operation creates its own alias.
func TestOperationResponseAliasConflictWithComponentResponse(t *testing.T) {
	cfg := Configuration{
		PackageName: "api",
		Output: &Output{
			UseSingleFile: true,
		},
	}

	codes, err := Generate([]byte(readTestdata(t, "response_alias_conflict.yml")), cfg)
	require.NoError(t, err)

	code := codes.GetCombined()

	// The operation "zone" references "OK", so it creates "ZoneResponse = OK"
	assert.Contains(t, code, "type ZoneResponse = OK")

	// The operation "getZones" references "Zone" response (which references Zone schema),
	// so it creates "GetZonesResponse = Zone"
	assert.Contains(t, code, "type GetZonesResponse = Zone")

	// The Zone schema should be generated as a struct
	assert.Contains(t, code, "type Zone struct")

	// Verify that the code compiles
	_, err = format.Source([]byte(code))
	require.NoError(t, err, "Generated code should compile without syntax errors")
}

// TestOperationResponseAliasReusesSameType tests that when an operation response alias
// would conflict with a component response name but they reference the same type,
// the existing alias is reused instead of creating a new one.
func TestOperationResponseAliasReusesSameType(t *testing.T) {
	cfg := Configuration{
		PackageName: "api",
		Output: &Output{
			UseSingleFile: true,
		},
	}

	codes, err := Generate([]byte(readTestdata(t, "response_alias_reuse.yml")), cfg)
	require.NoError(t, err)

	code := codes.GetCombined()

	// The component response "Zone" should be renamed to "ZoneResponse" due to conflict with schema "Zone"
	assert.Contains(t, code, "type ZoneResponse = Zone")

	// The operation "zone" also references "Zone" response, so it should reuse "ZoneResponse"
	// and NOT create "ZoneResponse1"
	assert.NotContains(t, code, "type ZoneResponse1")

	// Verify that the code compiles
	_, err = format.Source([]byte(code))
	require.NoError(t, err, "Generated code should compile without syntax errors")
}

func TestAssignWithResponseTypeNames(t *testing.T) {
	t.Run("empty operations slice is a no-op", func(t *testing.T) {
		tracker := newTypeTracker()
		var ops []OperationDefinition
		assignWithResponseTypeNames(ops, tracker)
		assert.Empty(t, ops)
	})

	t.Run("operation without response headers gets WithResponseTypeName but no HeaderTypeNames", func(t *testing.T) {
		tracker := newTypeTracker()
		ops := []OperationDefinition{{
			ID: "uploadDocument",
			Response: ResponseDefinition{
				Successes: []*ResponseContentDefinition{
					{StatusCode: 201, IsSuccess: true},
				},
			},
		}}

		assignWithResponseTypeNames(ops, tracker)

		assert.Equal(t, "UploadDocumentResp", ops[0].WithResponseTypeName)
		assert.Nil(t, ops[0].HeaderTypeNames)
	})

	t.Run("headers on success and error both get header type names", func(t *testing.T) {
		tracker := newTypeTracker()
		ops := []OperationDefinition{{
			ID: "uploadDocument",
			Response: ResponseDefinition{
				Successes: []*ResponseContentDefinition{
					{StatusCode: 201, IsSuccess: true, Headers: map[string]GoSchema{
						"Location": {GoType: "string"},
					}},
					// no headers on this one
					{StatusCode: 202, IsSuccess: true},
				},
				Errors: []*ResponseContentDefinition{
					{StatusCode: 422, Headers: map[string]GoSchema{
						"Retry-After": {GoType: "string"},
					}},
				},
			},
		}}

		assignWithResponseTypeNames(ops, tracker)

		require.NotNil(t, ops[0].HeaderTypeNames)
		assert.Equal(t, "UploadDocumentResp201Headers", ops[0].HeaderTypeNames[201])
		_, has202 := ops[0].HeaderTypeNames[202]
		assert.False(t, has202, "202 has no spec headers, so no Headers202 type should be reserved")
		assert.Equal(t, "UploadDocumentResp422Headers", ops[0].HeaderTypeNames[422])
	})

	t.Run("colliding wrapper name is disambiguated against existing tracker entries", func(t *testing.T) {
		tracker := newTypeTracker()
		// Simulate a user-declared schema that collides with the natural wrapper
		// name we'd otherwise generate.
		tracker.registerName("UploadDocumentResp")

		ops := []OperationDefinition{{
			ID: "uploadDocument",
			Response: ResponseDefinition{
				Successes: []*ResponseContentDefinition{{StatusCode: 201, IsSuccess: true}},
			},
		}}

		assignWithResponseTypeNames(ops, tracker)

		assert.NotEqual(t, "UploadDocumentResp", ops[0].WithResponseTypeName,
			"wrapper name must avoid the existing schema; got the colliding name back")
		assert.NotEmpty(t, ops[0].WithResponseTypeName)
	})

	t.Run("colliding header name is disambiguated", func(t *testing.T) {
		tracker := newTypeTracker()
		tracker.registerName("UploadDocumentResp201Headers")

		ops := []OperationDefinition{{
			ID: "uploadDocument",
			Response: ResponseDefinition{
				Successes: []*ResponseContentDefinition{
					{StatusCode: 201, IsSuccess: true, Headers: map[string]GoSchema{
						"Location": {GoType: "string"},
					}},
				},
			},
		}}

		assignWithResponseTypeNames(ops, tracker)

		assert.NotEqual(t, "UploadDocumentResp201Headers", ops[0].HeaderTypeNames[201],
			"header name must avoid the existing schema; got the colliding name back")
		assert.NotEmpty(t, ops[0].HeaderTypeNames[201])
	})

	t.Run("two operations get independent names registered with the tracker", func(t *testing.T) {
		tracker := newTypeTracker()
		ops := []OperationDefinition{
			{ID: "uploadDocument", Response: ResponseDefinition{
				Successes: []*ResponseContentDefinition{{StatusCode: 201, IsSuccess: true}},
			}},
			{ID: "deleteDocument", Response: ResponseDefinition{
				Successes: []*ResponseContentDefinition{{StatusCode: 204, IsSuccess: true}},
			}},
		}

		assignWithResponseTypeNames(ops, tracker)

		assert.Equal(t, "UploadDocumentResp", ops[0].WithResponseTypeName)
		assert.Equal(t, "DeleteDocumentResp", ops[1].WithResponseTypeName)
		// Both names must be present in the tracker so subsequent unique-name
		// resolution doesn't reuse them.
		assert.True(t, tracker.Exists("UploadDocumentResp"))
		assert.True(t, tracker.Exists("DeleteDocumentResp"))
	})
}

func TestOverlayAppliesExtensions(t *testing.T) {
	cfg := Configuration{
		PackageName: "api",
		Overlay: &OverlayOptions{
			Sources: []string{"testdata/overlay-add-extensions.yml"},
		},
	}

	codes, err := Generate([]byte(readTestdata(t, "overlay-base.yml")), cfg)
	require.NoError(t, err)

	code := codes.GetCombined()

	// The overlay adds x-go-name: UserModel to the User schema
	assert.Contains(t, code, "type UserModel struct")
	assert.NotContains(t, code, "type User struct")

	// The overlay adds x-go-name: UserID to the id property
	assert.Contains(t, code, "UserID")
}

func TestOverlayRemovesPath(t *testing.T) {
	cfg := Configuration{
		PackageName: "api",
		Overlay: &OverlayOptions{
			Sources: []string{"testdata/overlay-remove-internal.yml"},
		},
		Generate: &GenerateOptions{
			Client: true,
		},
	}

	codes, err := Generate([]byte(readTestdata(t, "overlay-base.yml")), cfg)
	require.NoError(t, err)

	code := codes.GetCombined()

	// The overlay removes /internal/health path
	assert.NotContains(t, code, "HealthCheck")
	assert.NotContains(t, code, "healthCheck")

	// But /users should still be there
	assert.Contains(t, code, "GetUsers")
}

func TestOverlayMultipleSources(t *testing.T) {
	cfg := Configuration{
		PackageName: "api",
		Overlay: &OverlayOptions{
			Sources: []string{
				"testdata/overlay-add-extensions.yml",
				"testdata/overlay-remove-internal.yml",
			},
		},
		Generate: &GenerateOptions{
			Client: true,
		},
	}

	codes, err := Generate([]byte(readTestdata(t, "overlay-base.yml")), cfg)
	require.NoError(t, err)

	code := codes.GetCombined()

	// First overlay: x-go-name applied
	assert.Contains(t, code, "type UserModel struct")

	// Second overlay: internal path removed
	assert.NotContains(t, code, "HealthCheck")

	// GetUsers should still exist
	assert.Contains(t, code, "GetUsers")
}

func TestOverlayInvalidSource(t *testing.T) {
	cfg := Configuration{
		PackageName: "api",
		Overlay: &OverlayOptions{
			Sources: []string{"testdata/nonexistent-overlay.yml"},
		},
	}

	_, err := Generate([]byte(readTestdata(t, "overlay-base.yml")), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error applying overlays")
}

// TestRawContentTypesGenerateByteSlice tests that raw content types (XML, YAML, etc.)
// generate []byte response types instead of structs, since we can't automatically
// unmarshal these formats.
func TestRawContentTypesGenerateByteSlice(t *testing.T) {
	cfg := Configuration{
		PackageName: "api",
		Output: &Output{
			UseSingleFile: true,
		},
		Generate: &GenerateOptions{
			Client: true,
		},
	}

	codes, err := Generate([]byte(readTestdata(t, "raw-content-types.yml")), cfg)
	require.NoError(t, err)

	code := codes.GetCombined()

	// Raw content types (YAML, XML) should generate []byte aliases
	assert.Contains(t, code, "type GetYamlConfigResponse = []byte")
	assert.Contains(t, code, "type GetXMLDataResponse = []byte")

	// JSON content type should still generate a struct
	assert.Contains(t, code, "type GetJSONDataResponse struct")

	// Client code for raw types should use direct byte conversion, not json.Unmarshal
	assert.Contains(t, code, "result := GetYamlConfigResponse(bodyBytes)")
	assert.Contains(t, code, "result := GetXMLDataResponse(bodyBytes)")

	// Client code for JSON should still use json.Unmarshal
	assert.Contains(t, code, "json.Unmarshal(bodyBytes, target)")

	// Verify that the code compiles
	_, err = format.Source([]byte(code))
	require.NoError(t, err, "Generated code should compile without syntax errors")
}

func TestSequentialContentTypesGenerateStreamSiblings(t *testing.T) {
	cfg := Configuration{
		PackageName: "api",
		Output: &Output{
			UseSingleFile: true,
		},
		Generate: &GenerateOptions{
			Client:             true,
			ClientWithResponse: true,
			ClientStreaming:    true,
		},
	}

	codes, err := Generate([]byte(readTestdata(t, "streaming.yml")), cfg)
	require.NoError(t, err)

	code := codes.GetCombined()

	// The non-streaming methods keep their media type and signature, whether or
	// not the operation also documents a sequential response.
	assert.Contains(t, code, "func (c *Client) GetEvents(ctx context.Context, reqEditors ...runtime.RequestEditorFn) (*GetEventsResponse, error)")
	assert.Contains(t, code, "type GetEventsResponse = []byte")
	assert.Contains(t, code, "type ChatResponse = Completion")

	// Streaming siblings carry the per-frame type: a $ref reuses the component,
	// an inline schema becomes <OperationID>ResponseItem, and a schema that
	// cannot be JSON-decoded falls back to raw frames.
	assert.Contains(t, code, "func (c *Client) GetEventsStream(ctx context.Context, reqEditors ...runtime.RequestEditorFn) (*runtime.Stream[Event], error)")
	assert.Contains(t, code, "func (c *Client) StreamLogsStream(ctx context.Context, reqEditors ...runtime.RequestEditorFn) (*runtime.Stream[StreamLogsResponseItem], error)")
	assert.Contains(t, code, "(*runtime.Stream[[]byte], error)")
	assert.Contains(t, code, "type StreamLogsResponseItem struct")

	// An operation declaring both application/json and text/event-stream at one
	// status exposes both shapes - no media type is displaced.
	assert.Contains(t, code, "func (c *Client) Chat(ctx context.Context, options *ChatRequestOptions, reqEditors ...runtime.RequestEditorFn) (*ChatResponse, error)")
	assert.Contains(t, code, "func (c *Client) ChatStream(ctx context.Context, options *ChatRequestOptions, reqEditors ...runtime.RequestEditorFn) (*runtime.Stream[Chunk], error)")
	assert.Contains(t, code, "JSON200      *ChatResponse")
	assert.Contains(t, code, "Stream200    *runtime.Stream[Chunk]")

	// The request advertises the media type and is marked so ExecuteRequest
	// leaves the body unread; framing follows the media type.
	// gofmt aligns struct keys per literal, so match without fixed spacing.
	assert.Regexp(t, `Stream:\s+"text/event-stream"`, code)
	assert.Regexp(t, `Stream:\s+"application/x-ndjson"`, code)
	assert.Contains(t, code, "return runtime.NewEventStream[Event](resp.Raw), nil")
	assert.Contains(t, code, "out.Stream200 = runtime.NewLineStream[StreamLogsResponseItem](resp.Raw)")

	// A `default` response standing in as the success is sequential too, and
	// is reached by a different code path than an explicit status.
	assert.Contains(t, code, "func (c *Client) FeedStream(ctx context.Context, reqEditors ...runtime.RequestEditorFn) (*runtime.Stream[Event], error)")

	// A plain JSON operation gains nothing.
	assert.NotContains(t, code, "ListUsersStream")

	_, err = format.Source([]byte(code))
	require.NoError(t, err, "Generated code should compile without syntax errors")
}

func TestSequentialContentTypesAreOptIn(t *testing.T) {
	newCfg := func(streaming bool) Configuration {
		return Configuration{
			PackageName: "api",
			Output:      &Output{UseSingleFile: true},
			Generate: &GenerateOptions{
				Client:             true,
				ClientWithResponse: true,
				ClientStreaming:    streaming,
			},
		}
	}

	off, err := Generate([]byte(readTestdata(t, "streaming.yml")), newCfg(false))
	require.NoError(t, err)
	on, err := Generate([]byte(readTestdata(t, "streaming.yml")), newCfg(true))
	require.NoError(t, err)

	offCode, onCode := off.GetCombined(), on.GetCombined()

	// With the flag off nothing streaming-related is emitted at all - not the
	// sibling methods, not the envelope fields, and not the per-frame item
	// types. That is what keeps the change non-breaking on upgrade.
	assert.NotContains(t, offCode, "runtime.Stream")
	assert.NotContains(t, offCode, "ResponseItem")
	assert.NotContains(t, offCode, "Stream:")

	// And the flag only ever adds: every method of the off output survives.
	for _, line := range strings.Split(offCode, "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "func (c *Client)") {
			assert.Contains(t, onCode, trimmed, "enabling client-streaming must not change existing methods")
		}
	}
}

func TestSequentialContentTypesUseItemSchema(t *testing.T) {
	cfg := Configuration{
		PackageName: "api",
		Output: &Output{
			UseSingleFile: true,
		},
		Generate: &GenerateOptions{
			Client:          true,
			ClientStreaming: true,
		},
	}

	codes, err := Generate([]byte(readTestdata(t, "streaming-item-schema.yml")), cfg)
	require.NoError(t, err)

	code := codes.GetCombined()

	// itemSchema describes one frame, so a $ref points at the component even
	// though the media type has no `schema` at all - and the component must
	// survive pruning, which only reaches it through itemSchema.
	assert.Contains(t, code, "type Chunk struct")
	assert.Contains(t, code, "(*runtime.Stream[Chunk], error)")

	// An inline itemSchema is generated exactly once.
	assert.Contains(t, code, "type InlineItemsResponseItem struct")
	assert.Equal(t, 1, strings.Count(code, "type InlineItemsResponseItem struct"))
	assert.Contains(t, code, "(*runtime.Stream[InlineItemsResponseItem], error)")

	_, err = format.Source([]byte(code))
	require.NoError(t, err, "Generated code should compile without syntax errors")
}

func TestSequentialContentTypesLeaveHandlerGenerationAlone(t *testing.T) {
	cfg := Configuration{
		PackageName: "api",
		Output: &Output{
			UseSingleFile: true,
		},
		Generate: &GenerateOptions{
			Handler:         &HandlerOptions{Kind: HandlerKindStdHTTP},
			ClientStreaming: true,
		},
	}

	codes, err := Generate([]byte(readTestdata(t, "streaming.yml")), cfg)
	require.NoError(t, err)

	code := codes.GetCombined()

	// Streaming lives entirely in the client templates, so the server side
	// never sees a stream type even with the flag on.
	assert.NotContains(t, code, "runtime.Stream[")
	assert.Contains(t, code, "type GetEventsResponse = []byte")

	_, err = format.Source([]byte(code))
	require.NoError(t, err, "Generated code should compile without syntax errors")
}

// captureLogs redirects the default slog logger for the duration of a test.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return buf
}

func TestSequentialContentTypesWarnWhenNotConsumable(t *testing.T) {
	tests := []struct {
		name            string
		generate        GenerateOptions
		expectBlocking  bool
		expectAvailable bool
	}{
		{
			name:     "client without streaming warns about blocking methods",
			generate: GenerateOptions{Client: true},
			// /events, /logs, /raw and /feed document nothing but a sequential
			// media type, so their generated methods block.
			expectBlocking: true,
			// /chat also declares application/json, so its method is fine - it
			// just cannot reach the stream.
			expectAvailable: true,
		},
		{
			name:            "envelope client without streaming warns too",
			generate:        GenerateOptions{ClientWithResponse: true},
			expectBlocking:  true,
			expectAvailable: true,
		},
		{
			name:            "streaming enabled says nothing",
			generate:        GenerateOptions{Client: true, ClientStreaming: true},
			expectBlocking:  false,
			expectAvailable: false,
		},
		{
			name:            "server-only generation says nothing",
			generate:        GenerateOptions{Handler: &HandlerOptions{Kind: HandlerKindStdHTTP}},
			expectBlocking:  false,
			expectAvailable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := captureLogs(t)

			generate := tt.generate
			_, err := Generate([]byte(readTestdata(t, "streaming.yml")), Configuration{
				PackageName: "api",
				Output:      &Output{UseSingleFile: true},
				Generate:    &generate,
			})
			require.NoError(t, err)

			out := logs.String()
			if tt.expectBlocking {
				assert.Contains(t, out, "will block")
				assert.Contains(t, out, "GET /events")
				assert.Contains(t, out, "GET /feed", "a `default` response standing in as the success counts too")
				assert.NotContains(t, out, `operations="POST /chat, `, "an operation with a buffered alternative does not block")
			} else {
				assert.NotContains(t, out, "will block")
			}

			if tt.expectAvailable {
				assert.Contains(t, out, "alongside a buffered one")
				assert.Contains(t, out, "POST /chat")
			} else {
				assert.NotContains(t, out, "alongside a buffered one")
			}
		})
	}
}

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
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist in the specification")
	})

	t.Run("succeeds with BasePath", func(t *testing.T) {
		cfg := NewDefaultConfiguration()
		cfg.BasePath = testdataDir

		doc, err := CreateDocument(contents, cfg)
		require.NoError(t, err)

		model, errs := doc.BuildV3Model()
		require.Empty(t, errs)

		getUserResp := model.Model.Paths.PathItems.GetOrZero("/users/{id}").Get.Responses.Codes.GetOrZero("200")
		require.NotNil(t, getUserResp)

		jsonContent := getUserResp.Content.GetOrZero("application/json")
		require.NotNil(t, jsonContent)

		schema := jsonContent.Schema.Schema()
		require.NotNil(t, schema)

		addressProp := schema.Properties.GetOrZero("address")
		require.NotNil(t, addressProp)

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

func TestCollectWebhookDefinitions(t *testing.T) {
	t.Run("nil webhooks returns nothing", func(t *testing.T) {
		contents, err := os.ReadFile("testdata/prune-cat-dog.yml")
		require.NoError(t, err)

		doc, err := LoadDocumentFromContents(contents)
		require.NoError(t, err)

		model, err := doc.BuildV3Model()
		require.NoError(t, err)

		opts := ParseOptions{typeTracker: newTypeTracker(), visited: map[string]bool{}, model: &model.Model}
		typeDefs, schemas, err := collectWebhookDefinitions(&model.Model, opts)
		require.NoError(t, err)
		assert.Empty(t, typeDefs)
		assert.Empty(t, schemas)
	})

	t.Run("ref webhooks produce alias types", func(t *testing.T) {
		contents, err := os.ReadFile("testdata/webhooks-with-examples.yml")
		require.NoError(t, err)

		doc, err := LoadDocumentFromContents(contents)
		require.NoError(t, err)

		model, err := doc.BuildV3Model()
		require.NoError(t, err)

		opts := ParseOptions{typeTracker: newTypeTracker(), visited: map[string]bool{}, model: &model.Model}
		typeDefs, schemas, err := collectWebhookDefinitions(&model.Model, opts)
		require.NoError(t, err)

		assert.NotEmpty(t, typeDefs)
		assert.NotEmpty(t, schemas)

		// All types should be tagged as webhook
		for _, td := range typeDefs {
			assert.Equal(t, SpecLocationWebhook, td.SpecLocation, "type %s should have webhook SpecLocation", td.Name)
		}

		// Should have body and response aliases
		names := make(map[string]bool)
		for _, td := range typeDefs {
			names[td.Name] = true
		}
		assert.True(t, names["PaymentCreatedBody"], "should have PaymentCreatedBody")
		assert.True(t, names["PaymentCreatedResponse"], "should have PaymentCreatedResponse")
	})

	t.Run("inline webhook body produces struct type", func(t *testing.T) {
		contents, err := os.ReadFile("testdata/filter-webhooks.yml")
		require.NoError(t, err)

		doc, err := LoadDocumentFromContents(contents)
		require.NoError(t, err)

		model, err := doc.BuildV3Model()
		require.NoError(t, err)

		opts := ParseOptions{typeTracker: newTypeTracker(), visited: map[string]bool{}, model: &model.Model}
		typeDefs, _, err := collectWebhookDefinitions(&model.Model, opts)
		require.NoError(t, err)

		// All types should be tagged as webhook
		for _, td := range typeDefs {
			assert.Equal(t, SpecLocationWebhook, td.SpecLocation, "type %s should have webhook SpecLocation", td.Name)
		}
	})
}

// An operation whose requestBody is optional must accept a request that carries
// none: the decoder reports an empty body, and the handler leaves opts.Body nil
// so the request reaches the service. A required body keeps failing to decode.
func TestOptionalRequestBodyToleratesAnEmptyBody(t *testing.T) {
	cfg := Configuration{
		PackageName: "api",
		Output:      &Output{UseSingleFile: true},
		Generate: &GenerateOptions{
			Handler: &HandlerOptions{Kind: "chi"},
		},
	}

	codes, err := Generate([]byte(readTestdata(t, "optional-request-body.yml")), cfg)
	require.NoError(t, err)

	code := codes.GetCombined()

	assert.Contains(t, code, "case errors.Is(err, runtime.ErrRequestBodyEmpty):",
		"the optional body admits a request that carries none")
	assert.Equal(t, 1, strings.Count(code, "runtime.ErrRequestBodyEmpty"),
		"only the optional operation admits it")

	// The required body keeps the shape it had before, so no consumer's
	// generated output shifts for an operation this does not change.
	assert.Contains(t, code, "if err := a.jsonBodyDecoder(r.Body, &body); err != nil {")

	_, err = format.Source([]byte(code))
	require.NoError(t, err, "Generated code should compile without syntax errors")
}
