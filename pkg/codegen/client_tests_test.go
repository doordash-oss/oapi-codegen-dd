// Copyright 2026 DoorDash, Inc.
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
	"encoding/json"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateClientTests(t *testing.T, spec string, cfg Configuration) GeneratedCode {
	t.Helper()
	code, err := Generate([]byte(spec), cfg)
	require.NoError(t, err)
	return code
}

func TestClientTests_Generated(t *testing.T) {
	cfg := NewDefaultConfiguration()
	cfg.PackageName = "api"
	cfg.Output = &Output{UseSingleFile: false}
	cfg.Generate.Client = true
	cfg.Generate.ClientTests = true

	code := generateClientTests(t, readTestdata(t, "client-tests.yml"), cfg)
	src, ok := code[clientTestsFileName]
	require.True(t, ok, "client_test file not generated")

	assert.Contains(t, src, "package api")
	assert.Contains(t, src, "func TestClientContract_UpdatePet(t *testing.T)")
	assert.Contains(t, src, "func TestClientContract_Ping(t *testing.T)")

	// The path parameter honors its minimum and the trailing slash is
	// dropped, as the client does.
	assert.Contains(t, src, `want := `+"`/pets/10`")
	assert.Contains(t, src, "`{\"petId\":10}`")
	// Primitive form parameters are checked by value, others by presence.
	assert.Contains(t, src, "got.query.Get(`dryRun`); v != `true`")
	assert.Contains(t, src, "k == `fields`")
	assert.Contains(t, src, "k == `filter`")
	// Headers honor minLength.
	assert.Contains(t, src, "got.header.Get(`X-Trace`); v != `fixtureaaaaa`")
	// The body and response fixtures use the schema's example and enum.
	assert.Contains(t, src, "`{\"kind\":\"dog\",\"name\":\"Rex\"}`")
	assert.Contains(t, src, `assertClientContractJSON(t, "request body", opts.Body, sent)`)
	// The documented error status is exercised.
	assert.Contains(t, src, `t.Run("error 422"`)
	// 204 has nothing to decode.
	assert.NotContains(t, src, "new(PingResponse)")

	assert.NotContains(t, code["client"], "httptest", "tests must stay out of the client")
}

func TestClientTests_SingleFile(t *testing.T) {
	cfg := NewDefaultConfiguration()
	cfg.PackageName = "api"
	cfg.Output = &Output{UseSingleFile: true, Filename: "gen.go"}
	cfg.Generate.Client = true
	cfg.Generate.ClientTests = true

	code := generateClientTests(t, readTestdata(t, "client-tests.yml"), cfg)
	assert.Contains(t, code[clientTestsFileName], "func TestClientContract_UpdatePet")
	assert.NotContains(t, code.GetCombined(), "httptest", "test code leaked into the combined output")
	assert.True(t, IsTestFile(clientTestsFileName))
	assert.False(t, IsTestFile("client"))
}

func TestClientTests_OffByDefault(t *testing.T) {
	cfg := NewDefaultConfiguration()
	cfg.Generate.Client = true

	code := generateClientTests(t, readTestdata(t, "client-tests.yml"), cfg)
	_, ok := code[clientTestsFileName]
	assert.False(t, ok)
}

func TestClientTests_RequireClient(t *testing.T) {
	cfg := NewDefaultConfiguration()
	cfg.Generate.Client = false
	cfg.Generate.ClientTests = true

	_, err := Generate([]byte(readTestdata(t, "client-tests.yml")), cfg)
	require.ErrorIs(t, err, ErrClientTestsRequireClient)
}

// componentSchema returns a resolved component schema from spec.
func componentSchema(t *testing.T, spec, name string) *base.Schema {
	t.Helper()
	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	model, err := doc.BuildV3Model()
	require.NoError(t, err)
	proxy, ok := model.Model.Components.Schemas.Get(name)
	require.True(t, ok, "schema %s not found", name)
	return proxy.Schema()
}

func fixtureOf(t *testing.T, spec, name string) map[string]any {
	t.Helper()
	js, err := fixtureJSON(componentSchema(t, spec, name))
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(js), &out), js)
	return out
}

func TestFixture_ScalarsHonorConstraints(t *testing.T) {
	spec := `
openapi: 3.1.0
info: {title: t, version: "1"}
components:
  schemas:
    S:
      type: object
      properties:
        date: {type: string, format: date}
        stamp: {type: string, format: date-time}
        id: {type: string, format: uuid}
        goUUID: {type: string, x-go-type: uuid.UUID}
        short: {type: string, maxLength: 3}
        count: {type: integer, minimum: 7, multipleOf: 5}
        below: {type: integer, maximum: 0}
        ratio: {type: number, exclusiveMinimum: 2}
        nullable: {type: [string, "null"]}
        pinned: {const: fixed}
        list: {type: array, minItems: 2, items: {type: string}}
        none: {type: array, maxItems: 0, items: {type: string}}
        bag: {type: object, additionalProperties: {type: integer}}
`
	got := fixtureOf(t, spec, "S")
	assert.Equal(t, "2026-01-02", got["date"])
	assert.Equal(t, "2026-01-02T03:04:05Z", got["stamp"])
	assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", got["id"])
	assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", got["goUUID"])
	assert.Equal(t, "fix", got["short"])
	assert.Equal(t, float64(10), got["count"])
	assert.Equal(t, float64(0), got["below"])
	assert.Equal(t, float64(3), got["ratio"])
	assert.Equal(t, "fixture", got["nullable"])
	assert.Equal(t, "fixed", got["pinned"])
	assert.Equal(t, []any{"fixture", "fixture1"}, got["list"])
	assert.Equal(t, []any{}, got["none"])
	assert.Equal(t, map[string]any{"key": float64(1)}, got["bag"])
}

func TestFixture_Unions(t *testing.T) {
	spec := `
openapi: 3.1.0
info: {title: t, version: "1"}
components:
  schemas:
    Cat: {type: object, properties: {kind: {type: string}, purrs: {type: boolean}}}
    Dog: {type: object, properties: {kind: {type: string, enum: [doggo]}, barks: {type: boolean}}}
    Mapped:
      oneOf: [{$ref: '#/components/schemas/Cat'}, {$ref: '#/components/schemas/Dog'}]
      discriminator: {propertyName: kind, mapping: {kitty: '#/components/schemas/Cat'}}
    Implicit:
      oneOf: [{$ref: '#/components/schemas/Cat'}]
      discriminator: {propertyName: kind}
    OwnValue:
      oneOf: [{$ref: '#/components/schemas/Dog'}]
      discriminator: {propertyName: kind}
    Both:
      anyOf: [{$ref: '#/components/schemas/Cat'}, {$ref: '#/components/schemas/Dog'}]
      oneOf: [{$ref: '#/components/schemas/Dog'}]
    Merged:
      allOf:
        - $ref: '#/components/schemas/Cat'
        - {type: object, required: [age], properties: {age: {type: integer}}}
`
	assert.Equal(t, "kitty", fixtureOf(t, spec, "Mapped")["kind"], "explicit mapping key")
	assert.Equal(t, "Cat", fixtureOf(t, spec, "Implicit")["kind"], "implicit mapping uses the schema name")
	assert.Equal(t, "doggo", fixtureOf(t, spec, "OwnValue")["kind"], "a variant's own enum wins over the schema name")
	assert.Contains(t, fixtureOf(t, spec, "Both"), "barks", "anyOf + oneOf picks a variant valid for both")
	merged := fixtureOf(t, spec, "Merged")
	assert.Contains(t, merged, "purrs")
	assert.Contains(t, merged, "age")
}

func TestFixture_CircularSchemasTerminate(t *testing.T) {
	spec := `
openapi: 3.1.0
info: {title: t, version: "1"}
components:
  schemas:
    Node:
      type: object
      required: [name]
      properties:
        name: {type: string}
        children: {type: array, items: {$ref: '#/components/schemas/Node'}}
        parent: {$ref: '#/components/schemas/Node'}
`
	got := fixtureOf(t, spec, "Node")
	assert.Equal(t, "fixture", got["name"])
	// The reference is followed once, then cut where it recurs.
	parent, ok := got["parent"].(map[string]any)
	require.True(t, ok, "parent: %v", got)
	assert.Equal(t, "fixture", parent["name"])
	assert.NotContains(t, parent, "parent")
	assert.Equal(t, []any{}, parent["children"])
}

func TestFixtureLiteral(t *testing.T) {
	assert.Equal(t, "`{\"a\":1}`", fixtureLiteral(`{"a":1}`))
	assert.Equal(t, "\"a`b\"", fixtureLiteral("a`b"))
	assert.Equal(t, `"a\rb"`, fixtureLiteral("a\rb"))
}

func TestFixture_Uint64AboveMaxInt64(t *testing.T) {
	spec := `
openapi: 3.1.0
info: {title: t, version: "1"}
components:
  schemas:
    S:
      type: object
      properties:
        big: {type: integer, format: uint64, example: 18446744073709551615}
        fits: {type: integer, format: uint64, example: 9223372036854775807}
`
	got := fixtureOf(t, spec, "S")
	// An example that does not fit in int64 must not wrap to a negative value.
	assert.Equal(t, float64(1), got["big"])
	assert.Equal(t, float64(9223372036854775807), got["fits"])
}

func TestFixture_GoTypeOverrideUsesExample(t *testing.T) {
	spec := `
openapi: 3.1.0
info: {title: t, version: "1"}
components:
  schemas:
    S:
      type: object
      properties:
        addr:
          type: string
          example: "192.0.2.10"
          x-go-type: netip.Addr
          x-go-type-import: {path: net/netip}
        stamp:
          type: string
          format: date-time
          example: "2020-05-06T07:08:09Z"
          x-go-type: time.Time
        noExample: {type: string, x-go-type: netip.Addr}
        formatOnly: {type: string, format: date, example: "not a date"}
`
	got := fixtureOf(t, spec, "S")
	assert.Equal(t, "192.0.2.10", got["addr"])
	assert.Equal(t, "2020-05-06T07:08:09Z", got["stamp"])
	assert.Equal(t, "192.0.2.1", got["noExample"])
	// Without x-go-type a formatted string still uses the format's fixture.
	assert.Equal(t, "2026-01-02", got["formatOnly"])
}

func TestIsGeneratedTestFile(t *testing.T) {
	assert.True(t, IsGeneratedTestFile("// Code generated by oapi-codegen. DO NOT EDIT.\n\npackage gen\n"))
	assert.True(t, IsGeneratedTestFile("// Copyright Acme\n\npackage gen\n\nfunc decodeClientContractFixture(t *testing.T) {}\n"))
	assert.False(t, IsGeneratedTestFile("package gen\n\nfunc TestMine(t *testing.T) {}\n"))
	assert.False(t, IsGeneratedTestFile("package gen\n\n// Code generated by oapi-codegen. DO NOT EDIT.\n"))
}
