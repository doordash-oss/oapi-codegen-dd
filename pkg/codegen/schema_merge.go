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

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
	"go.yaml.in/yaml/v4"
)

func createFromCombinator(schema *base.Schema, options ParseOptions) (GoSchema, error) {
	if schema == nil {
		return GoSchema{}, nil
	}

	path := options.path

	hasAllOf := len(schema.AllOf) > 0
	hasAnyOf := len(schema.AnyOf) > 0
	hasOneOf := len(schema.OneOf) > 0

	if !hasAllOf && !hasAnyOf && !hasOneOf {
		return GoSchema{}, nil
	}

	var (
		out             GoSchema
		allOfSchema     GoSchema
		anyOfSchema     GoSchema
		oneOfSchema     GoSchema
		additionalTypes []TypeDefinition
	)

	if hasAllOf {
		var err error
		allOfSchema, err = mergeAllOfSchemas(schema.AllOf, options)
		if err != nil {
			return GoSchema{}, fmt.Errorf("error merging allOf: %w", err)
		}

		// If the allOf resulted in a simple type (no properties), return it directly
		// This handles cases like allOf with a description-only schema and a $ref
		if len(allOfSchema.Properties) == 0 && !hasAnyOf && !hasOneOf {
			return allOfSchema, nil
		}

		out.Properties = append(out.Properties, allOfSchema.Properties...)
		additionalTypes = append(additionalTypes, allOfSchema.AdditionalTypes...)
	}

	if hasAnyOf {
		anyOfPath := append(path, "anyOf")
		var err error
		anyOfSchema, err = generateUnion(schema.AnyOf, nil, options.WithPath(anyOfPath))
		if err != nil {
			return GoSchema{}, fmt.Errorf("error resolving anyOf: %w", err)
		}

		// If generateUnion returned a simple type (not a union), return it directly
		// This happens when there's only 1 element, or when filtering null leaves 1 element
		if len(anyOfSchema.UnionElements) == 0 && !hasAllOf && !hasOneOf {
			return anyOfSchema, nil
		}

		anyOfFields := genFieldsFromProperties(anyOfSchema.Properties, options)
		anyOfSchema.GoType = anyOfSchema.createGoStruct(anyOfFields)

		anyOfName := pathToTypeName(anyOfPath)
		td := TypeDefinition{
			Name:             anyOfName,
			Schema:           anyOfSchema,
			SpecLocation:     SpecLocationUnion,
			JsonName:         "-",
			NeedsMarshaler:   needsMarshaler(anyOfSchema),
			HasSensitiveData: hasSensitiveData(anyOfSchema),
		}
		additionalTypes = append(additionalTypes, td)
		options.typeTracker.register(td, "")

		out.Properties = append(out.Properties, Property{
			GoName:      anyOfName,
			Schema:      GoSchema{RefType: anyOfName},
			Constraints: Constraints{Nullable: ptr(true)},
		})
	}

	if hasOneOf {
		oneOfPath := append(path, "oneOf")
		var err error
		oneOfSchema, err = generateUnion(schema.OneOf, schema.Discriminator, options.WithPath(oneOfPath))
		if err != nil {
			return GoSchema{}, fmt.Errorf("error resolving oneOf: %w", err)
		}

		// If generateUnion returned a simple type (not a union), return it directly
		// This happens when there's only 1 element, or when filtering null leaves 1 element
		if len(oneOfSchema.UnionElements) == 0 && !hasAllOf && !hasAnyOf {
			return oneOfSchema, nil
		}

		oneOfFields := genFieldsFromProperties(oneOfSchema.Properties, options)
		oneOfSchema.GoType = oneOfSchema.createGoStruct(oneOfFields)

		oneOfName := pathToTypeName(oneOfPath)
		td := TypeDefinition{
			Name:             oneOfName,
			Schema:           oneOfSchema,
			SpecLocation:     SpecLocationUnion,
			JsonName:         "-",
			NeedsMarshaler:   needsMarshaler(oneOfSchema),
			HasSensitiveData: hasSensitiveData(oneOfSchema),
		}
		additionalTypes = append(additionalTypes, td)
		options.typeTracker.register(td, "")

		out.Properties = append(out.Properties, Property{
			GoName:      oneOfName,
			Schema:      GoSchema{RefType: oneOfName},
			Constraints: Constraints{Nullable: ptr(true)},
		})
	}

	fields := genFieldsFromProperties(out.Properties, options)
	out.GoType = out.createGoStruct(fields)
	out.AdditionalTypes = append(out.AdditionalTypes, additionalTypes...)

	return out, nil
}

func containsUnion(schema *base.Schema) bool {
	if schema == nil {
		return false
	}

	if len(schema.AnyOf) > 0 || len(schema.OneOf) > 0 {
		return true
	}

	for _, s := range schema.AllOf {
		if containsUnion(s.Schema()) {
			return true
		}
	}
	return false
}

// isMetadataOnlySchema checks if a schema contains only metadata fields
// (description, title, examples, etc.) and no actual type/property definitions
func isMetadataOnlySchema(schema *base.Schema) bool {
	if schema == nil {
		return true
	}

	// If it has any of these, it's not metadata-only
	if len(schema.Type) > 0 {
		return false
	}
	if schema.Properties != nil && schema.Properties.Len() > 0 {
		return false
	}
	if schema.Items != nil {
		return false
	}
	if schema.AdditionalProperties != nil {
		return false
	}
	if len(schema.AllOf) > 0 || len(schema.AnyOf) > 0 || len(schema.OneOf) > 0 {
		return false
	}
	if schema.Not != nil {
		return false
	}

	// It only has metadata fields like description, title, examples, etc.
	return true
}

// mergeAllOfSchemas merges all the fields in the schemas supplied into one giant schema.
// The idea is that we merge all fields into one schema.
func mergeAllOfSchemas(allOf []*base.SchemaProxy, options ParseOptions) (GoSchema, error) {
	if len(allOf) == 0 {
		return GoSchema{}, nil
	}

	path := options.path

	allMergeable := true
	for _, s := range allOf {
		if containsUnion(s.Schema()) {
			allMergeable = false
			break
		}
	}

	if allMergeable {
		// Check if allOf contains only a single $ref with metadata (description, title, etc.)
		// If so, use the reference directly to avoid infinite recursion
		var refSchema *base.SchemaProxy
		var refCount int
		var metadataOnlyCount int

		for _, schemaProxy := range allOf {
			s := schemaProxy.Schema()
			ref := schemaProxy.GetReference()

			if ref != "" {
				refSchema = schemaProxy
				refCount++
			} else if isMetadataOnlySchema(s) {
				metadataOnlyCount++
			}
		}

		// If we have exactly one $ref and the rest are metadata-only schemas,
		// use the reference directly
		if refCount == 1 && refCount+metadataOnlyCount == len(allOf) {
			return GenerateGoSchema(refSchema, options.WithReference(refSchema.GetReference()))
		}

		var merged *base.Schema
		var lastRef string
		for _, schemaProxy := range allOf {
			s := schemaProxy.Schema()
			ref := schemaProxy.GetReference()

			// Track the last non-empty reference
			if ref != "" {
				lastRef = ref
			}

			var err error
			merged, err = mergeOpenapiSchemas(merged, s)
			if err != nil {
				return GoSchema{}, fmt.Errorf("error merging schemas for allOf: %w at path %v", err, path)
			}
		}

		schemaProxy := base.CreateSchemaProxy(merged)
		opts := options
		if low := schemaProxy.GoLow(); low != nil {
			opts = options.WithReference(low.GetReference())
		}

		// If we have a reference from one of the allOf elements and the merged schema
		// has no properties (i.e., it's a simple reference), use the reference.
		// This handles cases like allOf with a description-only schema and a $ref.
		if lastRef != "" && merged.Properties == nil {
			opts = options.WithReference(lastRef)
		}

		return GenerateGoSchema(schemaProxy, opts)
	}

	var (
		out             GoSchema
		additionalTypes []TypeDefinition
	)

	for i, schemaProxy := range allOf {
		subPath := append(path, fmt.Sprintf("allOf_%d", i))
		ref := schemaProxy.GoLow().GetReference()
		if ref != "" {
			typeName, err := refPathToGoType(ref)
			if err != nil {
				return GoSchema{}, fmt.Errorf("error converting reference to type name: %w", err)
			}

			// For path-based references (not component refs), we need to ensure the type is generated
			if !isStandardComponentReference(ref) {
				// Generate the schema to create type definitions for path-based references
				opts := options.WithReference(ref).WithPath(subPath)
				resolved, err := GenerateGoSchema(schemaProxy, opts)
				if err != nil {
					return GoSchema{}, fmt.Errorf("error resolving allOf[%d] path reference %s: %w", i, ref, err)
				}

				// Check if the type definition for the schema itself exists in additionalTypes
				typeExists := false
				for _, at := range resolved.AdditionalTypes {
					if at.Name == typeName {
						typeExists = true
						break
					}
				}

				// If the type doesn't exist, we need to create it
				if !typeExists && resolved.TypeDecl() != typeName {
					// The resolved schema has a different type name, so we need to create an alias or copy
					// For now, let's create a type definition with the expected name
					td := TypeDefinition{
						Name:             typeName,
						Schema:           resolved,
						SpecLocation:     SpecLocationUnion,
						NeedsMarshaler:   needsMarshaler(resolved),
						HasSensitiveData: hasSensitiveData(resolved),
					}
					// Add to type tracker if available
					if options.typeTracker != nil {
						options.typeTracker.register(td, "")
					}
					additionalTypes = append(additionalTypes, td)
				}

				// Add the additional types from the resolved schema
				additionalTypes = append(additionalTypes, resolved.AdditionalTypes...)
			}

			out.Properties = append(out.Properties, Property{
				GoName:      typeName,
				Schema:      GoSchema{RefType: typeName},
				Constraints: Constraints{Nullable: ptr(false)},
			})
			continue
		}

		// not a $ref - resolve as usual
		schema := schemaProxy.Schema()
		resolved, err := createFromCombinator(schema, options.WithPath(subPath))
		if err != nil {
			return GoSchema{}, fmt.Errorf("error resolving allOf[%d]: %w", i, err)
		}

		// Skip empty/zero schemas (e.g., schemas with properties but no type)
		if resolved.IsZero() {
			continue
		}

		fieldName := pathToTypeName(subPath)
		out.Properties = append(out.Properties, Property{
			GoName:      fieldName,
			Schema:      GoSchema{RefType: fieldName},
			Constraints: Constraints{Nullable: ptr(true)},
		})

		additionalTypes = append(additionalTypes, TypeDefinition{
			Name:             fieldName,
			Schema:           resolved,
			SpecLocation:     SpecLocationUnion,
			NeedsMarshaler:   needsMarshaler(resolved),
			HasSensitiveData: hasSensitiveData(resolved),
		})
		additionalTypes = append(additionalTypes, resolved.AdditionalTypes...)
	}

	out.GoType = out.createGoStruct(genFieldsFromProperties(out.Properties, options))

	td := TypeDefinition{
		Name:             pathToTypeName(path),
		Schema:           out,
		SpecLocation:     SpecLocationUnion,
		NeedsMarshaler:   needsMarshaler(out),
		HasSensitiveData: hasSensitiveData(out),
	}
	options.typeTracker.register(td, "")
	out.AdditionalTypes = append(out.AdditionalTypes, td)
	out.AdditionalTypes = append(out.AdditionalTypes, additionalTypes...)

	return out, nil
}

func mergeAllOf(allOf []*base.SchemaProxy) (*base.Schema, error) {
	var schema *base.Schema
	for _, schemaRef := range allOf {
		var err error
		schema, err = mergeOpenapiSchemas(schema, schemaRef.Schema())
		if err != nil {
			return nil, fmt.Errorf("error merging schemas for AllOf: %w", err)
		}
	}
	return schema, nil
}

// mergeOpenapiSchemas merges two openAPI schemas and returns the schema
// all of whose fields are composed.
func mergeOpenapiSchemas(s1, s2 *base.Schema) (*base.Schema, error) {
	if s1 == nil {
		// First schema, nothing to merge yet
		return s2, nil
	}

	result := &base.Schema{}

	t1 := getSchemaType(s1)
	t2 := getSchemaType(s2)

	// If a schema has no type, ignore it in the merge (e.g., description-only schemas)
	if len(t2) == 0 {
		return s1, nil
	}
	if len(t1) == 0 {
		return s2, nil
	}

	if !slices.Equal(t1, t2) {
		return nil, fmt.Errorf("can not merge incompatible types: %v, %v", t1, t2)
	}

	if s2.Extensions != nil && s2.Extensions.Len() > 0 {
		result.Extensions = orderedmap.New[string, *yaml.Node]()
		for k, v := range s2.Extensions.FromOldest() {
			// TODO: Check for collisions
			result.Extensions.Set(k, v)
		}
	}

	result.OneOf = append(s1.OneOf, s2.OneOf...)

	// We are going to make AllOf transitive, so that merging an AllOf that
	// contains AllOf's will result in a flat object.
	if s1.AllOf != nil {
		merged, err := mergeAllOf(s1.AllOf)
		if err != nil {
			return nil, ErrTransitiveMergingAllOfSchema1
		}
		s1 = merged
	}

	if s2.AllOf != nil {
		merged, err := mergeAllOf(s2.AllOf)
		if err != nil {
			return nil, ErrTransitiveMergingAllOfSchema2
		}
		s2 = merged
	}

	result.AllOf = append(s1.AllOf, s2.AllOf...)
	result.Type = t1

	if s1.Format != s2.Format {
		return nil, ErrMergingSchemasWithDifferentFormats
	}
	result.Format = s1.Format

	// For Enums, do we union, or intersect? This is a bit vague. I choose to be more permissive and union.
	result.Enum = append(s1.Enum, s2.Enum...)

	// not clear how to handle two different defaults.
	if s1.Default != nil || s2.Default != nil {
		return nil, ErrMergingSchemasWithDifferentDefaults
	}
	if s1.Default != nil {
		result.Default = s1.Default
	}
	if s2.Default != nil {
		result.Default = s2.Default
	}

	// If two schemas disagree on any of these flags, we error out.
	if s1.UniqueItems != s2.UniqueItems {
		return nil, ErrMergingSchemasWithDifferentUniqueItems
	}
	result.UniqueItems = s1.UniqueItems

	if s1.ExclusiveMinimum != s2.ExclusiveMinimum {
		return nil, ErrMergingSchemasWithDifferentExclusiveMin
	}
	result.ExclusiveMinimum = s1.ExclusiveMinimum

	if s1.ExclusiveMaximum != s2.ExclusiveMaximum {
		return nil, ErrMergingSchemasWithDifferentExclusiveMax
	}
	result.ExclusiveMaximum = s1.ExclusiveMaximum

	// For Nullable, we take the union (more permissive) approach:
	// - If either is true, result is true
	// - If both are false (or nil, which defaults to false), result is false
	// This allows merging schemas where one specifies nullable and another doesn't
	result.Nullable = mergeNullable(s1.Nullable, s2.Nullable)

	if !ptrEqual(s1.ReadOnly, s2.ReadOnly) {
		return nil, ErrMergingSchemasWithDifferentReadOnly
	}
	result.ReadOnly = s1.ReadOnly

	if !ptrEqual(s1.WriteOnly, s2.WriteOnly) {
		return nil, ErrMergingSchemasWithDifferentWriteOnly
	}
	result.WriteOnly = s1.WriteOnly

	// Required. We merge these.
	result.Required = append(s1.Required, s2.Required...)

	// We merge all properties
	for k, v := range s1.Properties.FromOldest() {
		if result.Properties == nil {
			result.Properties = orderedmap.New[string, *base.SchemaProxy]()
		}
		result.Properties.Set(k, v)
	}
	for k, v := range s2.Properties.FromOldest() {
		// TODO: detect conflicts
		if result.Properties == nil {
			result.Properties = orderedmap.New[string, *base.SchemaProxy]()
		}
		result.Properties.Set(k, v)
	}

	if isAdditionalPropertiesExplicitFalse(s1) || isAdditionalPropertiesExplicitFalse(s2) {
		result.AdditionalProperties = &base.DynamicValue[*base.SchemaProxy, bool]{
			A: nil,
			B: false,
		}
	} else if s1.AdditionalProperties != nil && s1.AdditionalProperties.IsA() && s1.AdditionalProperties.A != nil {
		if s2.AdditionalProperties != nil && s2.AdditionalProperties.A != nil {
			return nil, ErrMergingSchemasWithAdditionalProperties
		} else {
			result.AdditionalProperties = &base.DynamicValue[*base.SchemaProxy, bool]{
				A: s1.AdditionalProperties.A,
				B: true,
			}
		}
	} else {
		if s2.AdditionalProperties != nil && s2.AdditionalProperties.A != nil {
			result.AdditionalProperties = &base.DynamicValue[*base.SchemaProxy, bool]{
				A: s2.AdditionalProperties.A,
				B: true,
			}
		} else {
			if (s1.AdditionalProperties != nil && s1.AdditionalProperties.A != nil) ||
				(s2.AdditionalProperties != nil && s2.AdditionalProperties.A != nil) {
				result.AdditionalProperties = &base.DynamicValue[*base.SchemaProxy, bool]{
					A: nil,
					B: false,
				}
			}
		}
	}

	return result, nil
}

// isAdditionalPropertiesExplicitFalse determines whether an Schema is explicitly defined as `additionalProperties: false`
func isAdditionalPropertiesExplicitFalse(s *base.Schema) bool {
	if s.AdditionalProperties == nil {
		return false
	}

	return !s.AdditionalProperties.IsB()
}

func getSchemaType(schema *base.Schema) []string {
	if schema == nil {
		return nil
	}

	if schema.Type != nil {
		return schema.Type
	}

	if schema.Properties != nil {
		return []string{"object"}
	}

	if schema.Items != nil {
		return []string{"array"}
	}

	return nil
}

// mergeNullable merges two nullable pointers using a union (more permissive) approach.
// If either is true, the result is true. If both are false or nil, the result is nil.
// This allows merging schemas where one specifies nullable and another doesn't.
func mergeNullable(a, b *bool) *bool {
	// If either is explicitly true, result is true
	if (a != nil && *a) || (b != nil && *b) {
		return ptr(true)
	}
	// If both are nil or false, return nil (default behavior)
	return nil
}
