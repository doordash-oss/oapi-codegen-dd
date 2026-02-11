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
	"embed"
	"go/format"
	"os"
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
