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

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/pb33f/libopenapi/datamodel/high/base"
)

func createObjectSchema(schema *base.Schema, options ParseOptions) (GoSchema, error) {
	var (
		outType     string
		description string
		hasNilType  bool
	)
	if schema != nil {
		description = schema.Description
		hasNilType = slices.Contains(schema.Type, "null")
	}

	outSchema := GoSchema{
		Description:   description,
		OpenAPISchema: schema,
		Constraints: newConstraints(schema, ConstraintsContext{
			hasNilType:   hasNilType,
			specLocation: options.specLocation,
		}),
	}

	schemaExtensions := make(map[string]any)
	if schema != nil {
		schemaExtensions = extractExtensions(schema.Extensions)
	}

	path := options.path

	if schema != nil &&
		(schema.Properties == nil || schema.Properties.Len() == 0) &&
		!schemaHasAdditionalProperties(schema) &&
		schema.AllOf == nil &&
		schema.AnyOf == nil &&
		schema.OneOf == nil {
		t := schema.Type
		// If the object has no properties or additional properties, we
		// have some special cases for its type.
		if slices.Contains(t, "object") {
			// We have an object with no properties. This is a generic object
			// expressed as a map.
			outType = "map[string]any"
			outSchema.GoType = outType
			outSchema.DefineViaAlias = true
		} else { // t == ""
			// If we don't even have the object designator, we have an empty schema.
			// Use struct{} instead of any so we can define methods on it.
			// This is important for error responses and any other types that might need
			// to implement interfaces like error.
			outType = "struct{}"
			outSchema.GoType = outType
			outSchema.DefineViaAlias = false
		}
	} else {
		// When we define an object, we want it to be a type definition,
		// not a type alias, eg, "type Foo struct {...}"
		outSchema.DefineViaAlias = false
		var err error

		outSchema, err = enhanceSchemaWithAdditionalProperties(outSchema, schema, options)
		if err != nil {
			return GoSchema{}, err
		}

		// If the schema has no properties, and only additional properties, we will
		// early-out here and generate a map[string]<schema> instead of an object
		// that contains this map. We skip over anyOf/oneOf here because they can
		// introduce properties. allOf was handled above.
		if schema != nil &&
			(schema.Properties == nil || schema.Properties.Len() == 0) &&
			schema.AllOf == nil && schema.AnyOf == nil && schema.OneOf == nil {
			// We have a dictionary here. Returns the goType to be just a map from
			// string to the property type. HasAdditionalProperties=false means
			// that we won't generate custom json.Marshaler and json.Unmarshaler functions,
			// since we don't need them for a simple map.
			outSchema.HasAdditionalProperties = false
			outSchema.GoType = fmt.Sprintf("map[string]%s", additionalPropertiesType(outSchema))
			// Store the original OpenAPI schema so downstream tools can check if this
			// came from additionalProperties
			outSchema.OpenAPISchema = schema
			return outSchema, nil
		}

		// We've got an object with some properties.
		var required []string
		if schema != nil {
			required = schema.Required
		}

		// Track Go field names to detect conflicts
		goFieldNames := make(map[string]int)
		if schema != nil && schema.Properties != nil {
			for pName, p := range schema.Properties.FromOldest() {
				propertyPath := append(path, pName)
				pRef := p.GoLow().GetReference()
				opts := options.WithReference(pRef).WithPath(propertyPath)
				pSchema, err := GenerateGoSchema(p, opts)
				if err != nil {
					return GoSchema{}, fmt.Errorf("error generating Go schema for property '%s': %w", pName, err)
				}

				// Skip properties that have null-only type (type: "null")
				// These properties can only ever be null and cannot be represented in Go
				if pSchema.IsZero() {
					continue
				}

				hasNilTyp := false
				if p.Schema() != nil {
					hasNilTyp = slices.Contains(p.Schema().Type, "null")
				}
				constraints := newConstraints(p.Schema(), ConstraintsContext{
					name:         pName,
					hasNilType:   hasNilTyp,
					required:     slices.Contains(required, pName),
					specLocation: options.specLocation,
				})
				pSchema.Constraints = constraints

				if (pSchema.HasAdditionalProperties || len(pSchema.UnionElements) != 0) && pSchema.RefType == "" {
					// If we have fields present which have additional properties or union values,
					// but are not a pre-defined type, we need to define a type
					// for them, which will be based on the field names we followed
					// to get to the type.
					typeName := pathToTypeName(append(propertyPath, "AdditionalProperties"))

					// Use parent's SpecLocation if set, otherwise default to Schema or Union
					var specLocation = options.specLocation
					if specLocation == "" {
						specLocation = SpecLocationSchema
						if len(pSchema.UnionElements) != 0 {
							specLocation = SpecLocationUnion
						}
					}

					typeDef := TypeDefinition{
						Name:             typeName,
						JsonName:         strings.Join(propertyPath, "."),
						Schema:           pSchema,
						SpecLocation:     specLocation,
						HasSensitiveData: hasSensitiveData(pSchema),
					}
					options.AddType(typeDef)
					pSchema.AdditionalTypes = append(pSchema.AdditionalTypes, typeDef)
				}

				description := ""
				extensions := make(map[string]any)
				deprecated := false
				var sensitiveData *runtime.SensitiveDataConfig

				if p.Schema() != nil {
					s := p.Schema()
					description = s.Description
					extensions = extractExtensions(s.Extensions)
					if s.Deprecated != nil {
						deprecated = *s.Deprecated
					}

					// Parse x-sensitive-data extension
					if extension, ok := extensions[extSensitiveData]; ok {
						if config, err := extParseSensitiveData(extension); err == nil {
							sensitiveData = config
						}
					}
				}

				pSchema, _ = replaceInlineTypes(pSchema, opts)

				// Generate the Go field name and handle conflicts
				baseGoName := createPropertyGoFieldName(pName, extensions)
				goName := baseGoName
				if count, exists := goFieldNames[baseGoName]; exists {
					// Conflict detected - append a number
					count++
					goFieldNames[baseGoName] = count
					goName = fmt.Sprintf("%s%d", baseGoName, count)
				} else {
					goFieldNames[baseGoName] = 0
				}

				// Determine parent type name for recursive reference detection
				// Use the first element of the path as the parent type name
				parentType := ""
				if len(path) > 0 {
					parentType = pathToTypeName(path[:1])
				}

				prop := Property{
					GoName:        goName,
					JsonFieldName: pName,
					Schema:        pSchema,
					Description:   description,
					Extensions:    extensions,
					Deprecated:    deprecated,
					Constraints:   constraints,
					SensitiveData: sensitiveData,
					ParentType:    parentType,
				}
				outSchema.Properties = append(outSchema.Properties, prop)
				if len(pSchema.AdditionalTypes) > 0 {
					outSchema.AdditionalTypes = append(outSchema.AdditionalTypes, pSchema.AdditionalTypes...)
				}
			}
		}

		fields := genFieldsFromProperties(outSchema.Properties, options)
		outSchema.GoType = outSchema.createGoStruct(fields)

		// Check for x-go-type-name. It behaves much like x-go-type, however, it will
		// create a type definition for the named type, and use the named type in place
		// of this schema.
		if extension, ok := schemaExtensions[extGoTypeName]; ok {
			typeName, err := parseString(extension)
			if err != nil {
				return outSchema, fmt.Errorf("invalid value for %q: %w", extGoTypeName, err)
			}

			newTypeDef := TypeDefinition{
				Name:             typeName,
				Schema:           outSchema,
				SpecLocation:     SpecLocationSchema,
				HasSensitiveData: hasSensitiveData(outSchema),
			}
			options.AddType(newTypeDef)
			outSchema = GoSchema{
				Description:     newTypeDef.Schema.Description,
				GoType:          typeName,
				DefineViaAlias:  true,
				AdditionalTypes: append(outSchema.AdditionalTypes, newTypeDef),
			}
		}
	}

	return outSchema, nil
}

func enhanceSchemaWithAdditionalProperties(out GoSchema, schema *base.Schema, options ParseOptions) (GoSchema, error) {
	if schema == nil {
		return out, nil
	}

	if !schemaHasAdditionalProperties(schema) {
		return out, nil
	}

	path := options.path

	// If the schema has additional properties, we need to special case
	// a lot of behaviors.
	out.HasAdditionalProperties = true

	// Until we have a concrete additional properties type, we default to
	// any schema.
	out.AdditionalPropertiesType = &GoSchema{
		GoType: "any",
	}

	// If additional properties are defined, we will override the default
	// above with the specific definition.
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.IsA() {
		additionalSchema, err := GenerateGoSchema(schema.AdditionalProperties.A, options.WithPath(append(path, "AdditionalProperties")))
		if err != nil {
			return GoSchema{}, fmt.Errorf("error generating type for additional properties: %w", err)
		}

		if len(additionalSchema.Properties) > 0 || additionalSchema.HasAdditionalProperties || len(additionalSchema.UnionElements) != 0 {
			// If we have fields present which have additional properties or union values,
			// but are not a pre-defined type, we need to define a type
			// for them, which will be based on the field names we followed
			// to get to the type.
			typeName := pathToTypeName(append(path, "AdditionalProperties"))

			typeDef := TypeDefinition{
				Name:         typeName,
				JsonName:     strings.Join(append(path, "AdditionalProperties"), "."),
				Schema:       additionalSchema,
				SpecLocation: SpecLocationUnion,
			}
			options.AddType(typeDef)
			additionalSchema.RefType = typeName
			additionalSchema.AdditionalTypes = append(additionalSchema.AdditionalTypes, typeDef)
		}
		out.AdditionalPropertiesType = &additionalSchema
		out.AdditionalTypes = append(out.AdditionalTypes, additionalSchema.AdditionalTypes...)
	}

	return out, nil
}
