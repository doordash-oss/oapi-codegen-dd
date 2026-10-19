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
	"go/format"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSliceTypeDefinition(t *testing.T) {
	tracker := newTypeTracker()
	tracker.register(TypeDefinition{Name: "TagList", Schema: GoSchema{GoType: "[]string", ArrayType: &GoSchema{GoType: "string"}}}, "")
	tracker.register(TypeDefinition{Name: "Tags", Schema: GoSchema{GoType: "TagList", DefineViaAlias: true}}, "")
	tracker.register(TypeDefinition{Name: "Name", Schema: GoSchema{GoType: "string"}}, "")
	tracker.register(TypeDefinition{Name: "Self", Schema: GoSchema{GoType: "Self", DefineViaAlias: true}}, "")

	tests := []struct {
		name     string
		typeName string
		want     string
	}{
		{name: "slice type", typeName: "TagList", want: "TagList"},
		{name: "alias of a slice type", typeName: "Tags", want: "TagList"},
		{name: "not a slice", typeName: "Name"},
		{name: "unknown name", typeName: "[]string"},
		{name: "alias of itself", typeName: "Self"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := sliceTypeDefinition(tt.typeName, tracker)
			if tt.want == "" {
				assert.Nil(t, td)
				return
			}
			if assert.NotNil(t, td) {
				assert.Equal(t, tt.want, td.Name)
			}
		})
	}
}

func TestUnionParametersAreNamed(t *testing.T) {
	docContents, err := os.ReadFile("testdata/union-params.yml")
	require.NoError(t, err)

	codes, err := Generate(docContents, Configuration{
		PackageName: "api",
		Output:      &Output{UseSingleFile: true},
	})
	require.NoError(t, err)
	code := codes.GetCombined()

	_, err = format.Source([]byte(code))
	require.NoError(t, err)

	// An anonymous struct cannot carry the marshaler that merges the nameless
	// union field, so the parameter would be dropped from the request.
	assert.NotContains(t, code, "*struct {")

	t.Run("query union is a named type with a marshaler", func(t *testing.T) {
		assert.Contains(t, code, "ParentID *ListWidgetsQuery_ParentID ")
		assert.Contains(t, code, "func (l ListWidgetsQuery_ParentID) MarshalJSON()")
	})

	t.Run("header union is named too", func(t *testing.T) {
		assert.Contains(t, code, "func (l ListWidgetsHeaders_XOwner) MarshalJSON()")
	})

	t.Run("second operation gets its own type", func(t *testing.T) {
		assert.Contains(t, code, "func (l ListGadgetsQuery_ParentID) MarshalJSON()")
	})

	t.Run("single variant next to null stays a plain pointer", func(t *testing.T) {
		assert.NotContains(t, code, "ListWidgetsQuery_Order")
	})
}

func TestNameParamSchema(t *testing.T) {
	wrapper := GoSchema{
		Properties: []Property{{GoName: "Union", Schema: GoSchema{RefType: "Union"}}},
	}

	t.Run("registers a named type with a marshaler", func(t *testing.T) {
		options := ParseOptions{typeTracker: newTypeTracker()}

		got := nameParamSchema(wrapper, "ListQuery_ID", SpecLocationQuery, options)

		assert.Equal(t, "ListQuery_ID", got.RefType)
		require.Len(t, got.AdditionalTypes, 1)
		td := got.AdditionalTypes[0]
		assert.Equal(t, "ListQuery_ID", td.Name)
		assert.Equal(t, SpecLocationQuery, td.SpecLocation)
		assert.True(t, td.NeedsMarshaler)
		assert.True(t, options.typeTracker.Exists("ListQuery_ID"))
	})

	t.Run("name clash gets a unique name", func(t *testing.T) {
		options := ParseOptions{typeTracker: newTypeTracker()}
		options.typeTracker.register(TypeDefinition{Name: "ListQuery_ID"}, "")

		got := nameParamSchema(wrapper, "ListQuery_ID", SpecLocationQuery, options)

		assert.NotEqual(t, "ListQuery_ID", got.RefType)
		assert.Equal(t, got.RefType, got.AdditionalTypes[0].Name)
		assert.True(t, options.typeTracker.Exists(got.RefType))
	})

	t.Run("keeps nested types of the source schema", func(t *testing.T) {
		options := ParseOptions{typeTracker: newTypeTracker()}
		src := wrapper
		src.AdditionalTypes = []TypeDefinition{{Name: "Nested"}}

		got := nameParamSchema(src, "ListQuery_ID", SpecLocationQuery, options)

		require.Len(t, got.AdditionalTypes, 2)
		assert.Equal(t, "Nested", got.AdditionalTypes[0].Name)
		assert.Equal(t, "ListQuery_ID", got.AdditionalTypes[1].Name)
		assert.Len(t, src.AdditionalTypes, 1, "source schema must not be modified")
	})
}
