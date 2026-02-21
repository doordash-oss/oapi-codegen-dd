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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatCode(t *testing.T) {
	src := `
package main
import (
	"fmt"
	"os"
)
func main() {
fmt.Println("Hello, World!")
}
`

	expected := `package main

import (
	"fmt"
)

func main() {
	fmt.Println("Hello, World!")
}
`
	res, err := FormatCode(src)
	require.NoError(t, err)
	require.Equal(t, expected, res)
}

func Test_formatCode(t *testing.T) {
	src := `
package main
import (
	"fmt"
	"os"
)
func main() {
	fmt.Println("Hello, World!")
}
`

	expected := `package main

import (
	"fmt"
)

func main() {
	fmt.Println("Hello, World!")
}
`
	p := &Parser{cfg: Configuration{Output: &Output{SkipFmt: false}}}
	res, err := p.formatCode(src)
	require.NoError(t, err)
	require.Equal(t, expected, res)
}

func Test_formatCode_skip(t *testing.T) {
	// Unformatted code with unused imports
	src := `
package main
import (
	"fmt"
	"os"
)
func main() {
fmt.Println("Hello, World!")
}
`

	// When SkipFmt=true, no processing is done (no import optimization, no gofmt)
	p := &Parser{cfg: Configuration{Output: &Output{SkipFmt: true}}}
	res, err := p.formatCode(src)
	require.NoError(t, err)

	// Verify unused import "os" is NOT removed (imports not optimized)
	require.Contains(t, res, `"os"`)

	// Verify the code is returned as-is (gofmt skipped, bad indentation remains)
	require.Contains(t, res, "fmt.Println")
	require.NotContains(t, res, "\tfmt.Println")

	// When SkipFmt=false, full formatting is applied
	p = &Parser{cfg: Configuration{Output: &Output{SkipFmt: false}}}
	res, err = p.formatCode(src)
	require.NoError(t, err)

	// Verify unused import "os" IS removed
	require.NotContains(t, res, `"os"`)

	// gofmt fixes the indentation
	require.Contains(t, res, "\tfmt.Println")
}

func TestParser_Parse(t *testing.T) {
	parseOptions := ParseOptions{typeTracker: newTypeTracker()}
	cfg := Configuration{
		Output: &Output{
			UseSingleFile: true,
		},
	}

	t.Run("union types", func(t *testing.T) {
		unions := make([]TypeDefinition, 0)
		schema := GoSchema{
			RefType: "",
			UnionElements: []UnionElement{
				{TypeName: "int", Schema: GoSchema{GoType: "int"}},
				{TypeName: "string", Schema: GoSchema{GoType: "string"}},
				{TypeName: "bool", Schema: GoSchema{GoType: "bool"}},
			},
		}
		fields := genFieldsFromProperties(schema.Properties, parseOptions)
		schema.GoType = schema.createGoStruct(fields)

		td1 := TypeDefinition{
			Name:         "IntOrStringOrBool",
			SpecLocation: SpecLocationUnion,
			Schema:       schema,
		}
		unions = append(unions, td1)

		parseCtx := &ParseContext{
			UnionTypes: unions,
		}

		var expecteds []string

		expected1 := `
type IntOrStringOrBool struct {
	union json.RawMessage
}`
		expecteds = append(expecteds, expected1)

		parser, _ := NewParser(cfg, parseCtx)
		codes, err := parser.Parse()
		res := codes.GetCombined()

		require.NoError(t, err)

		for _, expected := range expecteds {
			assert.Contains(t, res, expected)
		}
	})

	t.Run("union type fields", func(t *testing.T) {
		unions := make([]TypeDefinition, 0)
		anyOfSchema := GoSchema{
			UnionElements: []UnionElement{
				{TypeName: "int", Schema: GoSchema{GoType: "int"}},
				{TypeName: "string", Schema: GoSchema{GoType: "string"}},
				{TypeName: "bool", Schema: GoSchema{GoType: "bool"}},
			},
		}
		anyOfFields := genFieldsFromProperties(anyOfSchema.Properties, parseOptions)
		anyOfSchema.GoType = anyOfSchema.createGoStruct(anyOfFields)

		anyOfTd := TypeDefinition{
			Name:   "IdAnyOf",
			Schema: anyOfSchema,
		}
		unions = append(unions, anyOfTd)

		oneOfSchema := GoSchema{
			UnionElements: []UnionElement{
				{TypeName: "int", Schema: GoSchema{GoType: "int"}},
				{TypeName: "string", Schema: GoSchema{GoType: "string"}},
			},
		}
		oneOfFields := genFieldsFromProperties(oneOfSchema.Properties, parseOptions)
		oneOfSchema.GoType = oneOfSchema.createGoStruct(oneOfFields)

		oneOfTd := TypeDefinition{
			Name:   "AddressOneOf",
			Schema: oneOfSchema,
		}
		unions = append(unions, oneOfTd)

		clientSchema := GoSchema{
			Properties: []Property{
				{
					GoName:        "IdAnyOf",
					JsonFieldName: "id",
					Schema:        GoSchema{RefType: "IdAnyOf"},
					Constraints:   Constraints{Nullable: ptr(true)},
				},
				{
					GoName:        "AddressOneOf",
					JsonFieldName: "address",
					Schema:        GoSchema{RefType: "AddressOneOf"},
					Constraints:   Constraints{Nullable: ptr(true)},
				},
			},
		}

		clientFields := genFieldsFromProperties(clientSchema.Properties, parseOptions)
		clientSchema.GoType = clientSchema.createGoStruct(clientFields)

		td := TypeDefinition{
			Name:     "Client",
			JsonName: "client",
			Schema:   clientSchema,
		}
		unions = append(unions, td)

		parseCtx := &ParseContext{
			UnionTypes: unions,
		}

		var expecteds []string

		expected1 := `
type Client struct {
	IdAnyOf      *IdAnyOf      ` + "`json:\"id,omitempty\"`" + `
	AddressOneOf *AddressOneOf ` + "`json:\"address,omitempty\"`" + `
}`
		expected2 := `
type IdAnyOf struct {
	union json.RawMessage
}`
		expected3 := `
type AddressOneOf struct {
	runtime.Either[int, string]
}`
		expecteds = append(expecteds, expected1, expected2, expected3)

		parser, _ := NewParser(cfg, parseCtx)
		codes, err := parser.Parse()
		res := codes.GetCombined()

		require.NoError(t, err)

		for i, expected := range expecteds {
			assert.Contains(t, res, expected, "failed expected %d", i+1)
		}
	})
}

func TestFilterAmbiguousRoutes(t *testing.T) {
	tests := []struct {
		name     string
		ops      []OperationDefinition
		expected []string // expected remaining paths
	}{
		{
			name: "no conflicts",
			ops: []OperationDefinition{
				{Method: "GET", Path: "/users"},
				{Method: "GET", Path: "/users/{id}"},
				{Method: "POST", Path: "/users"},
			},
			expected: []string{"/users", "/users/{id}", "/users"},
		},
		{
			name: "different methods no conflict",
			ops: []OperationDefinition{
				{Method: "GET", Path: "/queues/{id}/position"},
				{Method: "POST", Path: "/queues/functions/{id}"},
			},
			expected: []string{"/queues/{id}/position", "/queues/functions/{id}"},
		},
		{
			name: "ambiguous wildcard patterns",
			ops: []OperationDefinition{
				{Method: "GET", Path: "/queues/{requestId}/position"},
				{Method: "GET", Path: "/queues/functions/{functionId}"},
			},
			expected: []string{"/queues/{requestId}/position"}, // second one filtered
		},
		{
			name: "multiple conflicts keeps first",
			ops: []OperationDefinition{
				{Method: "GET", Path: "/api/{version}/users"},
				{Method: "GET", Path: "/api/v1/{resource}"},
				{Method: "GET", Path: "/api/{name}/config"},
			},
			expected: []string{"/api/{version}/users"}, // others conflict with first
		},
		{
			name: "catch-all wildcard conflicts",
			ops: []OperationDefinition{
				{Method: "GET", Path: "/files/{path...}"},
				{Method: "GET", Path: "/files/special"},
			},
			expected: []string{"/files/{path...}"}, // catch-all matches /files/special, conflict
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterAmbiguousRoutes(tt.ops, "test")
			var paths []string
			for _, op := range result {
				paths = append(paths, op.Path)
			}
			assert.Equal(t, tt.expected, paths)
		})
	}
}

func TestHasRouteConflict(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		conflict bool
	}{
		{
			name:     "both literal same",
			a:        []string{"users", "list"},
			b:        []string{"users", "list"},
			conflict: true,
		},
		{
			name:     "both literal different",
			a:        []string{"users", "list"},
			b:        []string{"users", "detail"},
			conflict: false,
		},
		{
			name:     "wildcard vs literal same prefix",
			a:        []string{"queues", "{id}", "position"},
			b:        []string{"queues", "functions", "{id}"},
			conflict: true,
		},
		{
			name:     "different lengths no catchall",
			a:        []string{"users"},
			b:        []string{"users", "{id}"},
			conflict: false,
		},
		{
			name:     "catchall vs longer",
			a:        []string{"files", "{path...}"},
			b:        []string{"files", "a", "b"},
			conflict: true,
		},
		{
			name:     "both wildcards",
			a:        []string{"api", "{version}", "users"},
			b:        []string{"api", "{name}", "users"},
			conflict: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasRouteConflict(tt.a, tt.b)
			assert.Equal(t, tt.conflict, result)
		})
	}
}

func TestHasDuplicateParamNames(t *testing.T) {
	tests := []struct {
		path      string
		duplicate bool
	}{
		{"/users", false},
		{"/users/{id}", false},
		{"/users/{id}/posts/{postId}", false},
		{"/teams/{id}/members/{id}", true},         // duplicate "id"
		{"/a/{x}/b/{y}/c/{x}", true},               // duplicate "x"
		{"/items/{id}/sub/{id}/detail/{id}", true}, // multiple duplicates
		{"/users/{userId}/posts/{postId}", false},  // different names
		{"/{org}/{repo}/issues/{id}", false},       // all different
		{"/{id}/{id}", true},                       // adjacent duplicates
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := hasDuplicateParamNames(tt.path)
			assert.Equal(t, tt.duplicate, result, "path: %s", tt.path)
		})
	}
}

func TestFilterRouteConflicts_DuplicateParamNames(t *testing.T) {
	ops := []OperationDefinition{
		{Method: "GET", Path: "/users/{id}"},
		{Method: "GET", Path: "/teams/{id}/members/{id}"}, // duplicate - filtered
		{Method: "GET", Path: "/users/{userId}/posts/{postId}"},
		{Method: "POST", Path: "/a/{x}/b/{x}"}, // duplicate - filtered
	}

	// Test with chi (non-std-http, only filters duplicates)
	result := filterRouteConflicts(ops, HandlerKindChi)

	expected := []string{"/users/{id}", "/users/{userId}/posts/{postId}"}
	var paths []string
	for _, op := range result {
		paths = append(paths, op.Path)
	}
	assert.Equal(t, expected, paths)
}
