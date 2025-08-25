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
	"path/filepath"
	"slices"
	"github.com/doordash/oapi-codegen/v3/pkg/codegen"
	"github.com/doordash/oapi-codegen/v3/pkg/codegen/ast"
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

func TestOptimizeImports(t *testing.T) {
	src := `
package main
import (
	"fmt"
	"foo"
	"bar"
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
	res, err := optimizeImports([]byte(src))
	require.NoError(t, err)
	require.Equal(t, expected, string(res))
}

func TestParser_Parse(t *testing.T) {
	parseOptions := ParseOptions{}
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
				"int",
				"string",
				"bool",
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
				"int",
				"string",
				"bool",
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
				"int",
				"string",
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
					Constraints:   Constraints{Nullable: true},
				},
				{
					GoName:        "AddressOneOf",
					JsonFieldName: "address",
					Schema:        GoSchema{RefType: "AddressOneOf"},
					Constraints:   Constraints{Nullable: true},
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
