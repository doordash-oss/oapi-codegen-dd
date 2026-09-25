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
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplicableErrorMapping(t *testing.T) {
	logs := captureLogs(t)

	cfg := Configuration{
		PackageName: "api",
		Output:      &Output{UseSingleFile: true},
		Generate: &GenerateOptions{
			Client:  true,
			Handler: &HandlerOptions{Kind: HandlerKindStdHTTP},
		},
		ErrorMapping: map[string]string{
			"PlainError":     "message",
			"UnionError":     "message",
			"TaggedError":    "message",
			"EnvelopeError":  "error.message",
			"UntaggedError":  "message",
			"UnmatchedError": "message",
			"Node":           "message",
			"ErrorVariant":   "message",
			"AliasedError":   "message",
			"NestedError":    "error.mesage",
		},
	}

	codes, err := Generate([]byte(readTestdata(t, "error-mapping.yml")), cfg)
	require.NoError(t, err)
	code := codes.GetCombined()
	warnings := logs.String()

	t.Run("struct paths get Error() and a constructor", func(t *testing.T) {
		assert.Contains(t, code, "func NewPlainError(message string) PlainError")
		assert.Contains(t, code, "NewPlainError(err.Error())")
		assert.NotContains(t, warnings, "type=PlainError ")
	})

	t.Run("union paths switch on the decoded variant", func(t *testing.T) {
		assert.Contains(t, code, `func (s UnionError) Error() string {
	res0 := s.UnionError_AnyOf
	if res0 == nil {
		return "unknown error"
	}
	res1 := *res0
	switch res2 := res1.Value().(type) {
	case ErrorVariant:
		res3 := res2.Message
		return res3
	}
	return "unknown error"
}`)
		assert.Contains(t, code, "res2, _ := res1.ValueByDiscriminator()")
		assert.Contains(t, code, "res2 := res1.EnvelopeError_Error_OneOf")

		for _, name := range []string{"UnionError", "TaggedError", "EnvelopeError"} {
			assert.NotContains(t, warnings, "type="+name+" ")
			// A message alone does not say which variant to build, so neither the
			// constructor nor a server adapter call to it may exist.
			assert.NotContains(t, code, "New"+name)
		}
	})

	t.Run("entries that cannot apply are reported", func(t *testing.T) {
		lines := strings.Split(warnings, "\n")
		tests := []struct {
			typeName string
			want     string
		}{
			{typeName: "UntaggedError", want: "more than two variants and no discriminator"},
			{typeName: "UnmatchedError", want: "neither does any variant of its oneOf/anyOf union"},
			{typeName: "Node", want: "neither does any variant of its oneOf/anyOf union"},
			{typeName: "ErrorVariant", want: "no error response type has this name"},
			{typeName: "AliasedError", want: "map BaseError instead"},
			{typeName: "NestedError", want: "'error' has no property 'mesage'"},
		}
		for _, tt := range tests {
			idx := slices.IndexFunc(lines, func(line string) bool {
				return strings.Contains(line, "level=WARN") && strings.Contains(line, "type="+tt.typeName+" ")
			})
			require.NotEqual(t, -1, idx, "expected a warning for %s", tt.typeName)
			assert.Contains(t, lines[idx], tt.want)
		}
	})

	t.Run("dropped entries are generated as unmapped", func(t *testing.T) {
		assert.Contains(t, code, "func (s UntaggedError) Error() string {\n\treturn \"unmapped client error\"\n}")
		assert.NotContains(t, code, "NewNestedError")
	})

	t.Run("caller's mapping is left intact", func(t *testing.T) {
		assert.Len(t, cfg.ErrorMapping, 10)
	})
}

func TestApplicableErrorMappingWithoutContext(t *testing.T) {
	mapping := map[string]string{"Error": "message"}

	assert.Equal(t, mapping, applicableErrorMapping(mapping, nil))
	assert.Equal(t, mapping, applicableErrorMapping(mapping, &ParseContext{}))
	assert.Nil(t, applicableErrorMapping(nil, &ParseContext{TypeTracker: newTypeTracker()}))
}

func TestApplicableErrorMappingDropsEntriesThatCannotApply(t *testing.T) {
	logs := captureLogs(t)

	plain := TypeDefinition{Name: "PlainError", Schema: GoSchema{Properties: []Property{
		{GoName: "Message", JsonFieldName: "message", Schema: GoSchema{GoType: "string"}},
	}}}
	other := TypeDefinition{Name: "OtherError", Schema: plain.Schema}
	tracker := newTypeTracker()
	for _, td := range []TypeDefinition{plain, other} {
		tracker.register(td, "")
		tracker.MarkNeedsErrorMethod(td.Name)
	}
	ctx := &ParseContext{
		TypeDefinitions: map[SpecLocation][]TypeDefinition{SpecLocationSchema: {plain, other}},
		TypeTracker:     tracker,
	}
	mapping := map[string]string{
		"PlainError": "message",
		"OtherError": "",
		"Unknown":    "message",
	}

	res := applicableErrorMapping(mapping, ctx)

	assert.Equal(t, map[string]string{"PlainError": "message"}, res)
	assert.Len(t, mapping, 3, "the caller's mapping is not modified")
	assert.Contains(t, logs.String(), `type=OtherError path="" reason="no error-mapping path"`)
	assert.Contains(t, logs.String(), "type=Unknown path=message")
}

func TestErrorConstructors(t *testing.T) {
	typ, typeSchemaMap := unionErrorType(GoSchema{UnionElements: []UnionElement{
		{TypeName: "NotFound"},
		{TypeName: "Conflict"},
	}})
	typeSchemaMap[typ.Name] = typ.Schema
	typeSchemaMap["PlainError"] = typeSchemaMap["NotFound"]

	res := errorConstructors(map[string]string{
		"PlainError": "message",
		"ResError":   "message",
		"Missing":    "message",
	}, typeSchemaMap)

	assert.Equal(t, map[string]bool{"PlainError": true}, res)
}

func TestResolveErrorPath(t *testing.T) {
	typ, typeSchemaMap := unionErrorType(GoSchema{UnionElements: []UnionElement{
		{TypeName: "NotFound"},
		{TypeName: "Gone"},
	}})
	typeSchemaMap["ErrorDetails"] = typeSchemaMap["NotFound"]
	envelope := GoSchema{Properties: []Property{
		{
			GoName:        "ErrorData",
			JsonFieldName: "error",
			Schema:        GoSchema{RefType: "ErrorDetails"},
			Constraints:   Constraints{Nullable: ptr(true)},
		},
	}}

	t.Run("follows a property that only carries a RefType", func(t *testing.T) {
		fields, err := resolveErrorPath("Envelope", map[string]string{"Envelope": "error.message"}, envelope, typeSchemaMap)
		require.NoError(t, err)
		require.Len(t, fields, 2)
		assert.Equal(t, "ErrorDetails", fields[0].goType)
		assert.Equal(t, "Message", fields[1].goName)
		assert.Equal(t, "ErrorDetails", fields[1].containerType)
	})

	t.Run("descends into an inline object", func(t *testing.T) {
		inline := GoSchema{Properties: []Property{{
			GoName:        "ErrorData",
			JsonFieldName: "error",
			Schema: GoSchema{GoType: "ErrorData", Properties: []Property{
				{GoName: "Message", JsonFieldName: "message", Schema: GoSchema{GoType: "string"}},
			}},
		}}}

		fields, err := resolveErrorPath("Inline", map[string]string{"Inline": "error.message"}, inline, typeSchemaMap)

		require.NoError(t, err)
		require.Len(t, fields, 2)
		assert.Equal(t, "ErrorData", fields[1].containerType)
	})

	t.Run("continues inside the variants of an embedded union", func(t *testing.T) {
		fields, err := resolveErrorPath(typ.Name, map[string]string{typ.Name: "message"}, typ.Schema, typeSchemaMap)
		require.NoError(t, err)
		require.Len(t, fields, 1)
		assert.Equal(t, "ResError_OneOf", fields[0].goName)
		require.NotNil(t, fields[0].union)
		require.Len(t, fields[0].union.variants, 1)
		assert.Equal(t, "NotFound", fields[0].union.variants[0].typeName)
	})

	errTests := []struct {
		name    string
		path    string
		schema  GoSchema
		wantMsg string
		wantErr error
	}{
		{
			name:    "no mapping path",
			path:    "",
			schema:  envelope,
			wantMsg: "no error-mapping path",
			wantErr: errNoErrorMappingPath,
		},
		{
			name:    "missing top-level property",
			path:    "mesage",
			schema:  envelope,
			wantMsg: "ResError has no property 'mesage'",
		},
		{
			name:    "missing nested property",
			path:    "error.mesage",
			schema:  envelope,
			wantMsg: "'error' has no property 'mesage'",
		},
		{
			name:    "no union variant has the path",
			path:    "code",
			schema:  typ.Schema,
			wantMsg: "ResError has no property 'code', and neither does any variant of its oneOf/anyOf union",
			wantErr: errNoUnionVariantHasPath,
		},
	}
	for _, tt := range errTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveErrorPath("ResError", map[string]string{"ResError": tt.path}, tt.schema, typeSchemaMap)
			require.Error(t, err)
			assert.Equal(t, tt.wantMsg, err.Error())
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

func TestResolveUnionVariants(t *testing.T) {
	_, typeSchemaMap := unionErrorType(GoSchema{})
	message := parseErrorPath("message")
	notFound := UnionElement{TypeName: "NotFound"}
	conflict := UnionElement{TypeName: "Conflict"}
	gone := UnionElement{TypeName: "Gone"}
	tagged := &Discriminator{
		Property: "_tag",
		Mapping:  map[string]string{"NotFound": "NotFound", "Conflict": "Conflict", "Gone": "Gone"},
	}

	t.Run("keeps only the variants that have the path", func(t *testing.T) {
		res, err := resolveUnionVariants("U", GoSchema{UnionElements: []UnionElement{notFound, gone}}, message, typeSchemaMap, map[string]bool{})
		require.NoError(t, err)
		require.Len(t, res.variants, 1)
		assert.Equal(t, "NotFound", res.variants[0].typeName)
	})

	t.Run("reads the variant through the discriminator unless there are exactly two", func(t *testing.T) {
		tests := []struct {
			elements []UnionElement
			want     bool
		}{
			{elements: []UnionElement{notFound}, want: true},
			{elements: []UnionElement{notFound, conflict}, want: false},
			{elements: []UnionElement{notFound, conflict, gone}, want: true},
		}
		for _, tt := range tests {
			res, err := resolveUnionVariants("U", GoSchema{UnionElements: tt.elements, Discriminator: tagged}, message, typeSchemaMap, map[string]bool{})
			require.NoError(t, err)
			assert.Equal(t, tt.want, res.discriminated, "%d variants", len(tt.elements))
		}
	})

	t.Run("needs a discriminator mapping for more than two variants", func(t *testing.T) {
		for _, discriminator := range []*Discriminator{nil, {Property: "_tag"}} {
			union := GoSchema{UnionElements: []UnionElement{notFound, conflict, gone}, Discriminator: discriminator}
			_, err := resolveUnionVariants("U", union, message, typeSchemaMap, map[string]bool{})
			assert.ErrorIs(t, err, errUnionVariantUnknown)
		}
	})

	t.Run("uses the inline schema of a variant that is not a named type", func(t *testing.T) {
		inline := UnionElement{TypeName: "Inline", Schema: typeSchemaMap["NotFound"]}
		res, err := resolveUnionVariants("U", GoSchema{UnionElements: []UnionElement{inline, gone}}, message, typeSchemaMap, map[string]bool{})
		require.NoError(t, err)
		require.Len(t, res.variants, 1)
		assert.Equal(t, "Inline", res.variants[0].typeName)
	})

	t.Run("fails when no variant has the path", func(t *testing.T) {
		str := UnionElement{TypeName: "string", Schema: GoSchema{GoType: "string"}}
		_, err := resolveUnionVariants("U", GoSchema{UnionElements: []UnionElement{gone, str}}, message, typeSchemaMap, map[string]bool{})
		assert.ErrorIs(t, err, errNoUnionVariantHasPath)
	})

	t.Run("stops at a union that contains itself", func(t *testing.T) {
		_, recursive := unionErrorType(GoSchema{})
		recursive["Node_OneOf"] = GoSchema{UnionElements: []UnionElement{gone, {TypeName: "Node"}}}
		recursive["Node"] = GoSchema{Properties: []Property{
			{GoName: "Node_OneOf", Schema: GoSchema{RefType: "Node_OneOf"}, Constraints: Constraints{Nullable: ptr(true)}},
		}}
		visited := map[string]bool{}

		_, err := resolveUnionVariants("Node_OneOf", recursive["Node_OneOf"], message, recursive, visited)

		assert.ErrorIs(t, err, errNoUnionVariantHasPath)
		assert.Empty(t, visited)
	})
}

func TestEmbeddedUnion(t *testing.T) {
	typ, typeSchemaMap := unionErrorType(GoSchema{UnionElements: []UnionElement{
		{TypeName: "NotFound"},
		{TypeName: "Gone"},
	}})

	t.Run("finds the embedded union field", func(t *testing.T) {
		schema := GoSchema{Properties: append(
			[]Property{{GoName: "Code", JsonFieldName: "code", Schema: GoSchema{GoType: "int"}}},
			typ.Schema.Properties...,
		)}

		prop, union, ok := embeddedUnion(schema, typeSchemaMap)

		require.True(t, ok)
		assert.Equal(t, "ResError_OneOf", prop.GoName)
		assert.Len(t, union.UnionElements, 2)
	})

	t.Run("ignores fields that are not embedded unions", func(t *testing.T) {
		tests := []struct {
			name string
			prop Property
		}{
			{name: "named union field", prop: Property{GoName: "Error", JsonFieldName: "error", Schema: GoSchema{RefType: "ResError_OneOf"}}},
			{name: "embedded struct", prop: Property{GoName: "NotFound", Schema: GoSchema{RefType: "NotFound"}}},
			{name: "unknown type", prop: Property{GoName: "Missing", Schema: GoSchema{RefType: "Missing"}}},
		}
		for _, tt := range tests {
			_, _, ok := embeddedUnion(GoSchema{Properties: []Property{tt.prop}}, typeSchemaMap)
			assert.False(t, ok, tt.name)
		}
	})
}

func TestErrorResponseCode(t *testing.T) {
	t.Run("numbers variables across nested unions", func(t *testing.T) {
		fields := []resolvedField{{
			goName:     "Outer",
			isNullable: true,
			union: &resolvedUnion{variants: []resolvedVariant{{
				typeName: "A",
				fields: []resolvedField{{
					goName:     "Inner",
					isNullable: true,
					union: &resolvedUnion{discriminated: true, variants: []resolvedVariant{{
						typeName: "B",
						fields:   []resolvedField{{goName: "Message"}},
					}}},
				}},
			}}},
		}}

		varIndex := 0
		res := strings.Join(errorResponseCode(fields, "e", &varIndex), "\n")
		expected := `res0 := e.Outer
if res0 == nil { return "unknown error" }
res1 := *res0
switch res2 := res1.Value().(type) {
case A:
res3 := res2.Inner
if res3 == nil { return "unknown error" }
res4 := *res3
res5, _ := res4.ValueByDiscriminator()
switch res6 := res5.(type) {
case B:
res7 := res6.Message
return res7
}
return "unknown error"
}
return "unknown error"`
		assert.Equal(t, expected, res)
	})
}

func TestErrorConstructorCode(t *testing.T) {
	tests := []struct {
		name   string
		fields []resolvedField
		want   string
	}{
		{
			name:   "single field",
			fields: []resolvedField{{goName: "Details"}},
			want:   "func NewResError(message string) ResError {\n\treturn ResError{Details: message}\n}",
		},
		{
			name:   "trailing array element",
			fields: []resolvedField{{goName: "Messages", isArray: true, arrayType: "string", isArrayIndex: true}},
			want:   "func NewResError(message string) ResError {\n\treturn ResError{Messages: []string{message}}\n}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, errorConstructorCode("ResError", tt.fields))
		})
	}
}

func TestReferencedTypeName(t *testing.T) {
	tests := []struct {
		name   string
		schema GoSchema
		want   string
	}{
		{name: "prefers GoType", schema: GoSchema{GoType: "ErrorDetails", RefType: "Other"}, want: "ErrorDetails"},
		{name: "falls back to RefType", schema: GoSchema{RefType: "Envelope_Error"}, want: "Envelope_Error"},
		{name: "names nothing", schema: GoSchema{}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, referencedTypeName(tt.schema))
		})
	}
}

func TestJoinErrorPath(t *testing.T) {
	assert.Equal(t, []errorPathSegment{
		{propertyName: "errors", isArrayIndex: true},
		{propertyName: "message"},
	}, parseErrorPath("errors[].message"))

	for _, path := range []string{"message", "error.message", "errors[].messages[]", "data[].detail.code"} {
		assert.Equal(t, path, joinErrorPath(parseErrorPath(path)))
	}
}

// unionErrorType returns an error type embedding union, the way oneOf/anyOf schemas are generated.
func unionErrorType(union GoSchema) (TypeDefinition, map[string]GoSchema) {
	typ := TypeDefinition{
		Name: "ResError",
		Schema: GoSchema{
			Properties: []Property{
				{
					GoName:      "ResError_OneOf",
					Schema:      GoSchema{RefType: "ResError_OneOf"},
					Constraints: Constraints{Nullable: ptr(true)},
				},
			},
		},
	}
	withMessage := GoSchema{Properties: []Property{
		{GoName: "Message", JsonFieldName: "message", Schema: GoSchema{GoType: "string"}},
	}}
	typeSchemaMap := map[string]GoSchema{
		"ResError_OneOf": union,
		"NotFound":       withMessage,
		"Conflict":       withMessage,
		"Gone": {Properties: []Property{
			{GoName: "Since", JsonFieldName: "since", Schema: GoSchema{GoType: "string"}},
		}},
	}
	return typ, typeSchemaMap
}
