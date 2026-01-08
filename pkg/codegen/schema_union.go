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

	"github.com/pb33f/libopenapi/datamodel/high/base"
)

// UnionElement describe a union element, based on the prefix externalRef\d+ and real ref name from external schema.
type UnionElement string

// Method generate union method name for template functions `As/From`.
func (u UnionElement) Method() string {
	var method string
	for _, part := range strings.Split(string(u), `.`) {
		method += UppercaseFirstCharacter(part)
	}
	return method
}

// goPrimitiveTypes is a set of Go primitive type names used to determine
// whether a type needs a separate type definition in unions.
var goPrimitiveTypes = map[string]bool{
	"string":  true,
	"int":     true,
	"int8":    true,
	"int16":   true,
	"int32":   true,
	"int64":   true,
	"uint":    true,
	"uint8":   true,
	"uint16":  true,
	"uint32":  true,
	"uint64":  true,
	"float":   true,
	"float32": true,
	"float64": true,
	"bool":    true,
}

func generateUnion(elements []*base.SchemaProxy, discriminator *base.Discriminator, options ParseOptions) (GoSchema, error) {
	outSchema := GoSchema{}
	path := options.path

	if discriminator != nil {
		outSchema.Discriminator = &Discriminator{
			Property: discriminator.PropertyName,
			Mapping:  make(map[string]string),
		}
	}

	// Early return for single element unions (no null involved)
	if len(elements) == 1 {
		ref := elements[0].GoLow().GetReference()
		opts := options.WithReference(ref).WithPath(options.path)
		return GenerateGoSchema(elements[0], opts)
	}

	// Filter out null types from union elements
	var nonNullElements []*base.SchemaProxy
	hasNull := false
	for _, element := range elements {
		if element == nil {
			continue
		}
		schema := element.Schema()
		if schema == nil {
			continue
		}
		// Check if this element is a null type
		if len(schema.Type) == 1 && slices.Contains(schema.Type, "null") {
			hasNull = true
			continue
		}
		nonNullElements = append(nonNullElements, element)
	}

	// If after filtering we have only 1 element, return it as a nullable type
	if len(nonNullElements) == 1 {
		ref := nonNullElements[0].GoLow().GetReference()
		opts := options.WithReference(ref).WithPath(options.path)
		schema, err := GenerateGoSchema(nonNullElements[0], opts)
		if err != nil {
			return GoSchema{}, err
		}
		if hasNull {
			schema.Constraints.Nullable = ptr(true)
		}
		return schema, nil
	}

	// Use the filtered elements for union generation
	elements = nonNullElements

	for i, element := range elements {
		if element == nil {
			continue
		}
		elementPath := append(path, fmt.Sprint(i))
		ref := element.GoLow().GetReference()
		opts := options.WithReference(ref).WithPath(elementPath)
		elementSchema, err := GenerateGoSchema(element, opts)
		if err != nil {
			return GoSchema{}, err
		}

		// define new types only for non-primitive types
		if ref == "" && !goPrimitiveTypes[elementSchema.GoType] {
			elementName := pathToTypeName(elementPath)
			if elementSchema.TypeDecl() != elementName {
				td := TypeDefinition{
					Schema:         elementSchema,
					Name:           elementName,
					SpecLocation:   SpecLocationUnion,
					JsonName:       "-",
					NeedsMarshaler: needsMarshaler(elementSchema),
				}
				options.AddType(td)
				outSchema.AdditionalTypes = append(outSchema.AdditionalTypes, td)
			}
			elementSchema.GoType = elementName
			outSchema.AdditionalTypes = append(outSchema.AdditionalTypes, elementSchema.AdditionalTypes...)
		} else if ref != "" && !goPrimitiveTypes[elementSchema.GoType] {
			// Handle path-based references (not component refs)
			// For path-based references to inline schemas, we need to create type definitions
			if !isStandardComponentReference(ref) && strings.HasPrefix(elementSchema.GoType, "struct") {
				elementName := pathToTypeName(elementPath)
				// Check if a type definition already exists
				typeExists := false
				for _, at := range elementSchema.AdditionalTypes {
					if at.Name == elementName {
						typeExists = true
						break
					}
				}

				if !typeExists {
					td := TypeDefinition{
						Schema:         elementSchema,
						Name:           elementName,
						SpecLocation:   SpecLocationUnion,
						JsonName:       "-",
						NeedsMarshaler: needsMarshaler(elementSchema),
					}
					options.AddType(td)
					outSchema.AdditionalTypes = append(outSchema.AdditionalTypes, td)
					elementSchema.GoType = elementName
				}
			}
			outSchema.AdditionalTypes = append(outSchema.AdditionalTypes, elementSchema.AdditionalTypes...)
		}

		if discriminator != nil {
			// Explicit mapping.
			var mapped bool
			for k, v := range discriminator.Mapping.FromOldest() {
				if v == element.GetReference() {
					outSchema.Discriminator.Mapping[k] = elementSchema.GoType
					mapped = true
					break
				}
			}
			// Implicit mapping.
			if !mapped {
				var discriminatorValue string

				// For inline schemas (no reference), try to extract the discriminator value
				if element.GetReference() == "" {
					discriminatorValue = extractDiscriminatorValue(element, discriminator.PropertyName)
					if discriminatorValue == "" {
						// If we have an explicit mapping but can't determine the discriminator value
						// for an inline schema, that's an error
						if discriminator.Mapping.Len() != 0 {
							return GoSchema{}, ErrAmbiguousDiscriminatorMapping
						}
						// Otherwise, skip this element (it won't be mapped)
						continue
					}
				} else {
					// For referenced schemas, use the reference name
					discriminatorValue = refPathToObjName(element.GetReference())
				}

				outSchema.Discriminator.Mapping[discriminatorValue] = elementSchema.GoType
			}
		}
		outSchema.UnionElements = append(outSchema.UnionElements, UnionElement(elementSchema.GoType))
	}

	// Deduplicate union elements to avoid generating duplicate methods
	outSchema.UnionElements = deduplicateUnionElements(outSchema.UnionElements)

	if (outSchema.Discriminator != nil) && len(outSchema.Discriminator.Mapping) != len(elements) {
		return GoSchema{}, ErrDiscriminatorNotAllMapped
	}

	return outSchema, nil
}

// deduplicateUnionElements removes duplicate union elements while preserving order
func deduplicateUnionElements(elements []UnionElement) []UnionElement {
	seen := make(map[UnionElement]bool)
	result := make([]UnionElement, 0, len(elements))

	for _, elem := range elements {
		if !seen[elem] {
			seen[elem] = true
			result = append(result, elem)
		}
	}

	return result
}

// extractDiscriminatorValue attempts to extract the discriminator value from a schema.
// For inline schemas, it looks for the discriminator property and extracts its enum value.
// Returns empty string if the value cannot be determined.
func extractDiscriminatorValue(element *base.SchemaProxy, discriminatorProp string) string {
	if element == nil {
		return ""
	}

	schema := element.Schema()
	if schema == nil || schema.Properties == nil {
		return ""
	}

	// Look for the discriminator property in the schema
	propProxy, found := schema.Properties.Get(discriminatorProp)
	if !found || propProxy == nil {
		return ""
	}

	propSchema := propProxy.Schema()
	if propSchema == nil || len(propSchema.Enum) == 0 {
		return ""
	}

	// Return the first (and typically only) enum value
	return propSchema.Enum[0].Value
}
