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
	"fmt"
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
// Schema is the GoSchema object used to populate the type description.
// SpecLocation indicates where in the OpenAPI spec this type was defined.
// NeedsMarshaler indicates whether this type needs a custom marshaler/unmarshaler.
// HasSensitiveData indicates whether this type has any properties marked as sensitive.
type TypeDefinition struct {
	Name             string
	JsonName         string
	Schema           GoSchema
	SpecLocation     SpecLocation
	NeedsMarshaler   bool
	HasSensitiveData bool
}

func (t TypeDefinition) IsAlias() bool {
	return t.Schema.DefineViaAlias
}

func (t TypeDefinition) IsOptional() bool {
	return t.Schema.Constraints.Required == nil || !*t.Schema.Constraints.Required
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

		if prop.Constraints.Nullable != nil && *prop.Constraints.Nullable {
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
