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

// UnionElement describes a union element with its type and schema (including constraints).
type UnionElement struct {
	// TypeName is the Go type name (e.g., "User", "string", "int")
	TypeName string

	// Schema contains the full schema including constraints (minLength, minimum, etc.)
	Schema GoSchema
}

// String returns the type name for backward compatibility with templates
func (u UnionElement) String() string {
	return u.TypeName
}

// Method generates union method name for template functions `As/From`.
func (u UnionElement) Method() string {
	var method string
	for _, part := range strings.Split(u.TypeName, `.`) {
		method += UppercaseFirstCharacter(part)
	}
	return method
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
		if ref == "" && !isPrimitiveType(elementSchema.GoType) {
			elementName := pathToTypeName(elementPath)
			if elementSchema.TypeDecl() != elementName {
				td := TypeDefinition{
					Schema:         elementSchema,
					Name:           elementName,
					SpecLocation:   SpecLocationUnion,
					JsonName:       "-",
					NeedsMarshaler: needsMarshaler(elementSchema),
				}
				options.typeTracker.register(td, "")
				outSchema.AdditionalTypes = append(outSchema.AdditionalTypes, td)
			}
			elementSchema.GoType = elementName
			outSchema.AdditionalTypes = append(outSchema.AdditionalTypes, elementSchema.AdditionalTypes...)
		} else if ref != "" && !isPrimitiveType(elementSchema.GoType) {
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
					options.typeTracker.register(td, "")
					outSchema.AdditionalTypes = append(outSchema.AdditionalTypes, td)
					elementSchema.GoType = elementName
				}
			}
			outSchema.AdditionalTypes = append(outSchema.AdditionalTypes, elementSchema.AdditionalTypes...)
		}

		if discriminator != nil {
			// Explicit mapping.
			// Note: We don't break after finding a match because the same reference
			// can appear multiple times in the oneOf with different discriminator values.
			// For example, TextValue might be mapped by both "text" and "filterable-text".
			var mapped bool
			for k, v := range discriminator.Mapping.FromOldest() {
				if v == element.GetReference() {
					outSchema.Discriminator.Mapping[k] = elementSchema.GoType
					mapped = true
					// Don't break - continue to find all mappings for this reference
				}
			}
			// Implicit mapping.
			if !mapped {
				var discriminatorValue string

				// Try to extract the discriminator value from the schema's enum first.
				// This works for both inline and referenced schemas.
				discriminatorValue = extractDiscriminatorValue(element, discriminator.PropertyName)

				if discriminatorValue == "" {
					if element.GetReference() == "" {
						// For inline schemas without an enum value, we can't determine the discriminator
						if discriminator.Mapping.Len() != 0 {
							return GoSchema{}, ErrAmbiguousDiscriminatorMapping
						}
						// Otherwise, skip this element (it won't be mapped)
						continue
					}
					// For referenced schemas without an enum value, fall back to the reference name
					discriminatorValue = refPathToObjName(element.GetReference())
				}

				outSchema.Discriminator.Mapping[discriminatorValue] = elementSchema.GoType
			}
		}
		// Skip struct{} elements - they have no data to marshal/unmarshal
		// and would generate invalid method names like AsStruct{}()
		if elementSchema.GoType == "struct{}" {
			continue
		}

		outSchema.UnionElements = append(outSchema.UnionElements, UnionElement{
			TypeName: elementSchema.GoType,
			Schema:   elementSchema,
		})
	}

	// Deduplicate union elements to avoid generating duplicate methods
	outSchema.UnionElements = deduplicateUnionElements(outSchema.UnionElements)

	if (outSchema.Discriminator != nil) && len(outSchema.Discriminator.Mapping) != len(elements) {
		return GoSchema{}, ErrDiscriminatorNotAllMapped
	}

	return outSchema, nil
}

// deduplicateUnionElements removes duplicate union elements while preserving order.
// When duplicates are found, it keeps the "stricter" one (the one with more validation constraints).
// If both have the same number of constraints, the first one wins.
func deduplicateUnionElements(elements []UnionElement) []UnionElement {
	seen := make(map[string]int) // maps TypeName to index in result
	result := make([]UnionElement, 0, len(elements))

	for _, elem := range elements {
		if existingIdx, found := seen[elem.TypeName]; !found {
			// First occurrence - add it
			seen[elem.TypeName] = len(result)
			result = append(result, elem)
		} else {
			// Duplicate found - keep the stricter one
			existing := result[existingIdx]
			if isStricterElement(elem, existing) {
				result[existingIdx] = elem
			}
		}
	}

	return result
}

// isStricterElement returns true if elem1 has more validation constraints than elem2.
// This helps us keep the more restrictive definition when deduplicating union elements.
func isStricterElement(elem1, elem2 UnionElement) bool {
	count1 := elem1.Schema.Constraints.Count()
	count2 := elem2.Schema.Constraints.Count()
	return count1 > count2
}

// extractDiscriminatorValue attempts to extract the discriminator value from a schema.
// It looks for the discriminator property and extracts its enum value.
// Works for both inline and referenced schemas (references are resolved automatically).
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

	// Only return the enum value if there's exactly one.
	// If there are multiple enum values, we can't determine which one is the
	// correct discriminator value for this specific schema, so return empty
	// to fall back to using the reference name.
	if len(propSchema.Enum) != 1 {
		return ""
	}

	return propSchema.Enum[0].Value
}
