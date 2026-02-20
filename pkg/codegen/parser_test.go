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
