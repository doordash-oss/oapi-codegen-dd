package codegen

import (
	"fmt"
	"slices"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
)

// mergeSchemas merges all the fields in the schemas supplied into one giant schema.
// The idea is that we merge all fields together into one schema.
func mergeSchemas(allOf []*base.SchemaProxy, path []string, options ParseOptions) (GoSchema, error) {
	n := len(allOf)

	if n == 1 {
		ref := allOf[0].GoLow().GetReference()
		return GenerateGoSchema(allOf[0], ref, path, options)
	}

	schema := allOf[0].Schema()

	for i := 1; i < n; i++ {
		var err error
		oneOfSchema := allOf[i].Schema()
		schema, err = mergeOpenapiSchemas(schema, oneOfSchema, true)
		if err != nil {
			return GoSchema{}, fmt.Errorf("error merging schemas for AllOf: %w", err)
		}
	}

	// TODO: check if that's ok, previously panicked
	schemaProxy := base.CreateSchemaProxy(schema)
	ref := ""
	if low := schemaProxy.GoLow(); low != nil {
		ref = low.GetReference()
	}

	return GenerateGoSchema(schemaProxy, ref, path, options)
}

func mergeAllOf(allOf []*base.SchemaProxy) (*base.Schema, error) {
	var schema *base.Schema
	for _, schemaRef := range allOf {
		var err error
		schema, err = mergeOpenapiSchemas(schema, schemaRef.Schema(), true)
		if err != nil {
			return nil, fmt.Errorf("error merging schemas for AllOf: %w", err)
		}
	}
	return schema, nil
}

// mergeOpenapiSchemas merges two openAPI schemas and returns the schema
// all of whose fields are composed.
func mergeOpenapiSchemas(s1, s2 *base.Schema, allOf bool) (*base.Schema, error) {
	result := &base.Schema{
		// Properties: orderedmap.New[string, *base.SchemaProxy](),
		// Extensions: orderedmap.New[string, *yaml.Node](),
	}

	t1 := getSchemaType(s1)
	t2 := getSchemaType(s2)
	if !slices.Equal(t1, t2) {
		return nil, fmt.Errorf("can not merge incompatible types: %v, %v", s1.Type, s2.Type)
	}

	if t1 == nil && t2 == nil {
		return nil, nil
	} else if t1 == nil {
		return s2, nil
	} else if t2 == nil {
		return s1, nil
	}

	for k, v := range s2.Extensions.FromOldest() {
		// TODO: Check for collisions
		result.Extensions.Set(k, v)
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

	if s1.Nullable != s2.Nullable {
		return nil, ErrMergingSchemasWithDifferentNullable
	}
	result.Nullable = s1.Nullable

	if s1.ReadOnly != s2.ReadOnly {
		return nil, ErrMergingSchemasWithDifferentReadOnly
	}
	result.ReadOnly = s1.ReadOnly

	if s1.WriteOnly != s2.WriteOnly {
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

	// Allow discriminators for allOf merges, but disallow for one/anyOfs.
	if !allOf && (s1.Discriminator != nil || s2.Discriminator != nil) {
		return nil, ErrMergingSchemasWithDifferentDiscriminators
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
