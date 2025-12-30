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
	_ "embed"
	"go/format"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/test_spec.yml
var testDocument string

//go:embed testdata/user.yml
var userDocument string

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
	codes, err := Generate([]byte(testDocument), cfg)
	require.NoError(t, err)

	code := codes.GetCombined()
	assert.NotEmpty(t, code)

	// Check that we have a package:
	assert.Contains(t, code, "package testswagger")

	assert.Contains(t, code, "Top *int `json:\"$top,omitempty\"`")
	assert.Contains(t, code, "DeadSince *time.Time    `json:\"dead_since,omitempty\" tag1:\"value1\" tag2:\"value2\"`")
	assert.Contains(t, code, "type EnumTestNumerics int")
	assert.Contains(t, code, "EnumTestNumericsN2 EnumTestNumerics = 2")
	assert.Contains(t, code, "type EnumTestEnumNames int")
	assert.Contains(t, code, "EnumTestEnumNamesTwo  EnumTestEnumNames = 2")
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
	assert.Contains(t, code, "type N400_Issues struct")
	assert.NotContains(t, code, "type 400_Issues struct") // Should NOT have unprefixed version
	assert.NotContains(t, code, "[]400_Issues")           // Should NOT have unprefixed array type
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
