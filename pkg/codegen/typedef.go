package codegen

import (
	"fmt"
	"slices"
	"strings"
)

type SpecLocation string

const (
	SpecLocationPath     SpecLocation = "path"
	SpecLocationQuery    SpecLocation = "query"
	SpecLocationHeader   SpecLocation = "header"
	SpecLocationBody     SpecLocation = "body"
	SpecLocationResponse SpecLocation = "response"
	SpecLocationSchema   SpecLocation = "schema"
	SpecLocationUnion    SpecLocation = "union"
)

// TypeDefinition describes a Go type definition in generated code.
// Name is the name of the type in the schema, eg, type <...> Person.
// JsonName is the name of the corresponding JSON description, as it will sometimes
// differ due to invalid characters.
// GoSchema is the GoSchema object used to populate the type description.
type TypeDefinition struct {
	Name         string
	JsonName     string
	Schema       GoSchema
	SpecLocation SpecLocation
}

func (t TypeDefinition) IsAlias() bool {
	return t.Schema.DefineViaAlias
}

func (t TypeDefinition) IsOptional() bool {
	return !t.Schema.Constraints.Required
}

// GetErrorResponse generates a Go code snippet that returns an error response
// based on the predefined spec error path.
func (t TypeDefinition) GetErrorResponse(errTypes map[string]string, alias string) string {
	unknownRes := `return "unknown error"`

	key := t.Name
	path, ok := errTypes[key]
	if !ok || path == "" {
		return unknownRes
	}

	var (
		schema   = t.Schema
		callPath []keyValue[string, Property]
	)

	for _, part := range strings.Split(path, ".") {
		found := false
		for _, prop := range schema.Properties {
			if prop.JsonFieldName == part {
				callPath = append(callPath, keyValue[string, Property]{prop.GoName, prop})
				schema = prop.Schema
				found = true
				break
			}
		}
		if !found {
			return unknownRes
		}
	}

	if len(callPath) == 0 {
		return unknownRes
	}

	var (
		code     []string
		prevVar  = alias
		varName  string
		varIndex = 0
	)

	for _, pair := range callPath {
		name, prop := pair.key, pair.value

		varName = fmt.Sprintf("res%d", varIndex)
		code = append(code, fmt.Sprintf("%s := %s.%s", varName, prevVar, name))

		if prop.Constraints.Nullable {
			code = append(code, fmt.Sprintf("if %s == nil { %s }", varName, unknownRes))

			// Prepare for next access with dereference
			varIndex++
			derefVar := fmt.Sprintf("res%d", varIndex)
			code = append(code, fmt.Sprintf("%s := *%s", derefVar, varName))
			prevVar = derefVar
		} else {
			prevVar = varName
		}

		varIndex++
	}

	// Final field check
	lastProp := callPath[len(callPath)-1].value
	if lastProp.Schema.GoType != "string" {
		return unknownRes
	}

	code = append(code, fmt.Sprintf("return %s", prevVar))
	return strings.Join(code, "\n")
}

// TypeRegistry is a registry of type names.
type TypeRegistry map[string]int

// GetName returns a unique name for the given type name.
func (tr TypeRegistry) GetName(name string) string {
	if cnt, found := tr[name]; found {
		next := cnt + 1
		tr[name] = next
		return fmt.Sprintf("%s%d", name, next)
	}
	return name
}

func checkDuplicates(types []TypeDefinition) ([]TypeDefinition, error) {
	m := map[string]TypeDefinition{}
	var ts []TypeDefinition

	for _, typ := range types {
		if other, found := m[typ.Name]; found {
			// If type names collide, we need to see if they refer to the same
			// exact type definition, in which case, we can de-dupe.
			// If they don't match, we error out.
			if typeDefinitionsEquivalent(other, typ) {
				continue
			}
			// We want to create an error when we try to define the same type twice.
			return nil, fmt.Errorf("duplicate typename '%s' detected, can't auto-rename, "+
				"please use x-go-name to specify your own name for one of them", typ.Name)
		}

		m[typ.Name] = typ

		ts = append(ts, typ)
	}

	return ts, nil
}

// typeDefinitionsEquivalent checks for equality between two type definitions, but
// not every field is considered.
// We only want to know if they are fundamentally the same type.
func typeDefinitionsEquivalent(t1, t2 TypeDefinition) bool {
	if equal := t1.Name == t2.Name &&
		t1.Schema.TypeDecl() == t2.Schema.TypeDecl() &&
		slices.Equal(t1.Schema.UnionElements, t2.Schema.UnionElements) &&
		slices.Equal(t1.Schema.OpenAPISchema.Enum, t2.Schema.OpenAPISchema.Enum); equal {
		return true
	}

	for ix, prop := range t1.Schema.Properties {
		if !prop.IsEqual(t2.Schema.Properties[ix]) {
			return false
		}
	}
	return true
}
