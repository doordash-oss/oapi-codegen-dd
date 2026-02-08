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
// The path supports array access with [] suffix, e.g., "data[].message[]" will
// access the first element of each array.
func (t TypeDefinition) GetErrorResponse(errTypes map[string]string, alias string, typeSchemaMap map[string]GoSchema) string {
	unknownRes := `return "unknown error"`

	fields := resolveErrorPath(t.Name, errTypes, t.Schema, typeSchemaMap)
	if fields == nil {
		return unknownRes
	}

	var (
		code     []string
		prevVar  = alias
		varName  string
		varIndex = 0
	)

	for _, entry := range fields {
		varName = fmt.Sprintf("res%d", varIndex)
		code = append(code, fmt.Sprintf("%s := %s.%s", varName, prevVar, entry.goName))

		// For nullable non-array types, add nil check and dereference
		// For arrays, we handle nil check in the array access section (via len check)
		if entry.isNullable && !entry.isArray {
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

		// Handle array access
		if entry.isArrayIndex {
			code = append(code, fmt.Sprintf("if len(%s) == 0 { %s }", prevVar, unknownRes))

			varName = fmt.Sprintf("res%d", varIndex)
			code = append(code, fmt.Sprintf("%s := %s[0]", varName, prevVar))
			prevVar = varName
			varIndex++
		}
	}

	code = append(code, fmt.Sprintf("return %s", prevVar))
	return strings.Join(code, "\n")
}

// errorPathSegment represents a parsed segment of an error mapping path.
type errorPathSegment struct {
	propertyName string
	isArrayIndex bool
}

// parseErrorPath parses an error mapping path like "data[].message[]" into segments.
func parseErrorPath(path string) []errorPathSegment {
	parts := strings.Split(path, ".")
	segments := make([]errorPathSegment, 0, len(parts))

	for _, part := range parts {
		isArray := strings.HasSuffix(part, "[]")
		propName := strings.TrimSuffix(part, "[]")
		segments = append(segments, errorPathSegment{
			propertyName: propName,
			isArrayIndex: isArray,
		})
	}

	return segments
}

// resolvedField contains all info needed for both Get and Set error response methods.
type resolvedField struct {
	goName        string
	goType        string
	containerType string // The type of the struct that contains this field (for nested struct literals)
	isNullable    bool
	isArray       bool
	arrayType     string
	isArrayIndex  bool
	prop          Property
}

// resolveErrorPath traverses the schema following the error-mapping path and returns
// resolved field info for each segment. Returns nil if path not found or invalid.
func resolveErrorPath(typeName string, errTypes map[string]string, schema GoSchema, typeSchemaMap map[string]GoSchema) []resolvedField {
	path, ok := errTypes[typeName]
	if !ok || path == "" {
		return nil
	}

	segments := parseErrorPath(path)
	if len(segments) == 0 {
		return nil
	}

	fields := make([]resolvedField, 0, len(segments))
	// Track the current container type (the type of the struct we're looking at)
	currentContainerType := typeName

	for _, seg := range segments {
		found := false
		for _, prop := range schema.Properties {
			if prop.JsonFieldName == seg.propertyName {
				isNullable := prop.Constraints.Nullable != nil && *prop.Constraints.Nullable
				isArray := prop.Schema.ArrayType != nil

				f := resolvedField{
					goName:        prop.GoName,
					goType:        prop.Schema.GoType,
					containerType: currentContainerType,
					isNullable:    isNullable,
					isArray:       isArray,
					isArrayIndex:  seg.isArrayIndex,
					prop:          prop,
				}

				if isArray && prop.Schema.ArrayType != nil {
					f.arrayType = prop.Schema.ArrayType.GoType
				}

				fields = append(fields, f)
				schema = prop.Schema

				// If array access, get the element schema
				if seg.isArrayIndex && schema.ArrayType != nil {
					schema = *schema.ArrayType
				}

				// If the property references another type, resolve it
				if schema.GoType != "" && len(schema.Properties) == 0 {
					if resolvedSchema, ok := typeSchemaMap[schema.GoType]; ok {
						// Update container type to the referenced type before resolving
						currentContainerType = schema.GoType
						schema = resolvedSchema
					}
				} else if schema.GoType != "" {
					// The schema has properties already resolved - use its GoType
					currentContainerType = schema.GoType
				}

				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}

	if len(fields) == 0 {
		return nil
	}

	return fields
}
