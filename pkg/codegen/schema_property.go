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
	"slices"
	"strings"

	"github.com/doordash/oapi-codegen-dd/v3/pkg/runtime"
)

type Property struct {
	GoName        string
	Description   string
	JsonFieldName string
	Schema        GoSchema
	Extensions    map[string]any
	Deprecated    bool
	Constraints   Constraints
	SensitiveData *runtime.SensitiveDataConfig
}

func (p Property) IsEqual(other Property) bool {
	return p.JsonFieldName == other.JsonFieldName &&
		p.Schema.TypeDecl() == other.Schema.TypeDecl() &&
		p.Constraints.IsEqual(other.Constraints)
}

func (p Property) GoTypeDef() string {
	typeDef := p.Schema.TypeDecl()

	if p.IsPointerType() {
		typeDef = "*" + strings.TrimPrefix(typeDef, "*")
	}
	return typeDef
}

// IsPointerType returns true if this property's Go type is a pointer.
func (p Property) IsPointerType() bool {
	typeDef := p.Schema.TypeDecl()

	// Arrays, maps, and objects with additional properties are not pointers
	if p.Schema.OpenAPISchema != nil && slices.Contains(p.Schema.OpenAPISchema.Type, "array") {
		return false
	}
	if p.Schema.OpenAPISchema != nil && slices.Contains(p.Schema.OpenAPISchema.Type, "object") {
		if schemaHasAdditionalProperties(p.Schema.OpenAPISchema) {
			return false
		}
	}
	if strings.HasPrefix(typeDef, "map[") || strings.HasPrefix(typeDef, "[]") {
		return false
	}

	// Check if it's a pointer based on nullable and SkipOptionalPointer
	return !p.Schema.SkipOptionalPointer && p.Constraints.Nullable != nil && *p.Constraints.Nullable
}

// needsCustomValidation returns true if this property needs custom validation logic
// (i.e., calling Validate() method) instead of just using validator tags.
func (p Property) needsCustomValidation() bool {
	// Inline union (RefType with no JsonFieldName)
	if p.Schema.RefType != "" && p.JsonFieldName == "" {
		return true
	}

	// Property with union elements
	if len(p.Schema.UnionElements) > 0 {
		return true
	}

	// Named ref (RefType with JsonFieldName)
	if p.Schema.RefType != "" && p.JsonFieldName != "" {
		return true
	}

	// Custom Go type that's not a primitive
	if p.Schema.GoType != "" {
		typeDef := p.Schema.TypeDecl()

		// Inline arrays and maps of primitives don't need custom validation
		if strings.HasPrefix(typeDef, "[]") || strings.HasPrefix(typeDef, "map[") {
			return false
		}

		primitives := []string{"string", "int", "int32", "int64", "float32", "float64", "bool", "time.Time"}
		for _, prim := range primitives {
			if typeDef == prim {
				return false
			}
		}
		return true
	}

	return false
}

func createPropertyGoFieldName(jsonName string, extensions map[string]any) string {
	goFieldName := jsonName
	if extension, ok := extensions[extGoName]; ok {
		if extGoFieldName, err := parseString(extension); err == nil {
			goFieldName = extGoFieldName
		}
	}

	if extension, ok := extensions[extOapiCodegenOnlyHonourGoName]; ok {
		if use, err := parseBooleanValue(extension); err == nil {
			if use {
				return goFieldName
			}
		}
	}

	// convert some special names needed for interfaces
	if goFieldName == "error" {
		goFieldName = "ErrorData"
	}

	return schemaNameToTypeName(goFieldName)
}

// genFieldsFromProperties produce corresponding field names with JSON annotations,
// given a list of schema descriptors
func genFieldsFromProperties(props []Property, options ParseOptions) []string {
	var fields []string

	for i, p := range props {
		field := ""
		goFieldName := p.GoName

		// Add a comment to a field in case we have one, otherwise skip.
		if !options.OmitDescription && p.Description != "" {
			// Separate the comment from a previous-defined, unrelated field.
			// Make sure the actual field is separated by a newline.
			if i != 0 {
				field += "\n"
			}
			field += fmt.Sprintf("%s\n", stringWithTypeNameToGoComment(p.Description, p.GoName))
		}

		if p.Deprecated {
			// This comment has to be on its own line for godoc & IDEs to pick up
			var deprecationReason string
			if extension, ok := p.Extensions[extDeprecationReason]; ok {
				if extOmitEmpty, err := parseString(extension); err == nil {
					deprecationReason = extOmitEmpty
				}
			}

			field += fmt.Sprintf("%s\n", deprecationComment(deprecationReason))
		}

		// Check x-go-type-skip-optional-pointer, which will override if the type
		// should be a pointer or not when the field is optional.
		if extension, ok := p.Extensions[extPropGoTypeSkipOptionalPointer]; ok {
			if skipOptionalPointer, err := parseBooleanValue(extension); err == nil {
				p.Schema.SkipOptionalPointer = skipOptionalPointer
			}
		}

		field += fmt.Sprintf("    %s %s", goFieldName, p.GoTypeDef())

		c := p.Constraints
		omitEmpty := c.Nullable != nil && *c.Nullable
		if p.Schema.SkipOptionalPointer {
			omitEmpty = false
		}

		// Support x-omitempty
		if extOmitEmptyValue, ok := p.Extensions[extPropOmitEmpty]; ok {
			if extOmitEmpty, err := parseBooleanValue(extOmitEmptyValue); err == nil {
				omitEmpty = extOmitEmpty
			}
		}

		fieldTags := make(map[string]string)

		if len(p.Constraints.ValidationTags) > 0 {
			fieldTags["validate"] = strings.Join(c.ValidationTags, ",")
		}

		jsonFieldName := p.JsonFieldName
		if jsonFieldName == "" {
			jsonFieldName = "-"
		}
		fieldTags["json"] = jsonFieldName
		if omitEmpty && jsonFieldName != "-" {
			fieldTags["json"] += ",omitempty"
		}

		// Support x-go-json-ignore
		if extension, ok := p.Extensions[extPropGoJsonIgnore]; ok {
			if goJsonIgnore, err := parseBooleanValue(extension); err == nil && goJsonIgnore {
				fieldTags["json"] = "-"
			}
		}

		// Support x-oapi-codegen-extra-tags
		if extension, ok := p.Extensions[extPropExtraTags]; ok {
			if tags, err := extExtraTags(extension); err == nil {
				keys := sortedMapKeys(tags)
				for _, k := range keys {
					fieldTags[k] = tags[k]
				}
			}
		}

		// Support x-sensitive-data - add a simple marker tag
		// The actual masking is handled via custom MarshalJSON generation
		if _, ok := p.Extensions[extSensitiveData]; ok {
			fieldTags["sensitive"] = ""
		}

		// Convert the fieldTags map into Go field annotations.
		keys := sortedMapKeys(fieldTags)
		tags := make([]string, len(keys))
		for j, k := range keys {
			tags[j] = fmt.Sprintf(`%s:"%s"`, k, fieldTags[k])
		}
		field += "`" + strings.Join(tags, " ") + "`"
		fields = append(fields, field)
	}

	return fields
}
