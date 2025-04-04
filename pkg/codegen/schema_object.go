package codegen

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
	"gopkg.in/yaml.v3"
)

func createObjectSchema(schema *base.Schema, ref string, path []string) (GoSchema, error) {
	var outType string
	outSchema := GoSchema{
		Description:   schema.Description,
		OpenAPISchema: schema,
		Constraints: newConstraints(schema, ConstraintsContext{
			hasNilType: slices.Contains(schema.Type, "null"),
		}),
	}

	var schemaExtensions *orderedmap.Map[string, *yaml.Node]
	if schema != nil && schema.Extensions != nil {
		schemaExtensions = schema.Extensions
	} else {
		schemaExtensions = orderedmap.New[string, *yaml.Node]()
	}

	if schema != nil &&
		(schema.Properties == nil || schema.Properties.Len() == 0) &&
		!schemaHasAdditionalProperties(schema) &&
		schema.AnyOf == nil &&
		schema.OneOf == nil {
		t := schema.Type
		// If the object has no properties or additional properties, we
		// have some special cases for its type.
		if slices.Contains(t, "object") {
			// We have an object with no properties. This is a generic object
			// expressed as a map.
			outType = "map[string]any"
		} else { // t == ""
			// If we don't even have the object designator, we're a completely
			// generic type.
			outType = "any"
		}
		outSchema.GoType = outType
		outSchema.DefineViaAlias = true
	} else {
		// When we define an object, we want it to be a type definition,
		// not a type alias, eg, "type Foo struct {...}"
		outSchema.DefineViaAlias = false

		// If the schema has additional properties, we need to special case
		// a lot of behaviors.
		outSchema.HasAdditionalProperties = schemaHasAdditionalProperties(schema)

		// Until we have a concrete additional properties type, we default to
		// any schema.
		outSchema.AdditionalPropertiesType = &GoSchema{
			GoType: "any`",
		}

		// If additional properties are defined, we will override the default
		// above with the specific definition.
		if schema.AdditionalProperties != nil && schema.AdditionalProperties.IsA() {
			additionalSchema, err := GenerateGoSchema(schema.AdditionalProperties.A, ref, path)
			if err != nil {
				return GoSchema{}, fmt.Errorf("error generating type for additional properties: %w", err)
			}
			if additionalSchema.HasAdditionalProperties || len(additionalSchema.UnionElements) != 0 {
				// If we have fields present which have additional properties or union values,
				// but are not a pre-defined type, we need to define a type
				// for them, which will be based on the field names we followed
				// to get to the type.
				typeName := pathToTypeName(append(path, "AdditionalProperties"))
				typeName = schemaNameToTypeName(typeName)

				typeDef := TypeDefinition{
					Name:         typeName,
					JsonName:     strings.Join(append(path, "AdditionalProperties"), "."),
					Schema:       additionalSchema,
					SpecLocation: SpecLocationUnion,
				}
				additionalSchema.RefType = typeName
				additionalSchema.AdditionalTypes = append(additionalSchema.AdditionalTypes, typeDef)
			}
			outSchema.AdditionalPropertiesType = &additionalSchema
			outSchema.AdditionalTypes = append(outSchema.AdditionalTypes, additionalSchema.AdditionalTypes...)
		}

		// If the schema has no properties, and only additional properties, we will
		// early-out here and generate a map[string]<schema> instead of an object
		// that contains this map. We skip over anyOf/oneOf here because they can
		// introduce properties. allOf was handled above.
		if schema != nil &&
			(schema.Properties == nil || schema.Properties.Len() == 0) &&
			schema.AnyOf == nil && schema.OneOf == nil {
			// We have a dictionary here. Returns the goType to be just a map from
			// string to the property type. HasAdditionalProperties=false means
			// that we won't generate custom json.Marshaler and json.Unmarshaler functions,
			// since we don't need them for a simple map.
			outSchema.HasAdditionalProperties = false
			outSchema.GoType = fmt.Sprintf("map[string]%s", additionalPropertiesType(outSchema))
			return outSchema, nil
		}

		// We've got an object with some properties.
		required := schema.Required
		for pName, p := range schema.Properties.FromOldest() {
			propertyPath := append(path, pName)
			pRef := p.GoLow().GetReference()
			pSchema, err := GenerateGoSchema(p, pRef, propertyPath)
			if err != nil {
				return GoSchema{}, fmt.Errorf("error generating Go schema for property '%s': %w", pName, err)
			}

			hasNilType := false
			if p.Schema() != nil {
				hasNilType = slices.Contains(p.Schema().Type, "null")
			}
			constraints := newConstraints(p.Schema(), ConstraintsContext{
				name:       pName,
				hasNilType: hasNilType,
				required:   slices.Contains(required, pName),
			})
			pSchema.Constraints = constraints

			if (pSchema.HasAdditionalProperties || len(pSchema.UnionElements) != 0) && pSchema.RefType == "" {
				// If we have fields present which have additional properties or union values,
				// but are not a pre-defined type, we need to define a type
				// for them, which will be based on the field names we followed
				// to get to the type.
				typeName := pathToTypeName(propertyPath) // schemaNameToTypeName(pathToTypeName(propertyPath))
				specLocation := SpecLocation("")
				if len(pSchema.UnionElements) != 0 {
					specLocation = SpecLocationUnion
				}

				typeDef := TypeDefinition{
					Name:         typeName,
					JsonName:     strings.Join(propertyPath, "."),
					Schema:       pSchema,
					SpecLocation: specLocation,
				}
				pSchema.AdditionalTypes = append(pSchema.AdditionalTypes, typeDef)
				pSchema.RefType = typeName
			}

			description := ""
			extensions := make(map[string]any)
			deprecated := false

			if p.Schema() != nil {
				s := p.Schema()
				description = s.Description
				extensions = extractExtensions(s.Extensions)
				if s.Deprecated != nil {
					deprecated = *s.Deprecated
				}
			}

			prop := Property{
				GoName:        createPropertyGoFieldName(pName, extensions),
				JsonFieldName: pName,
				Schema:        pSchema,
				Description:   description,
				Extensions:    extensions,
				Deprecated:    deprecated,
				Constraints:   constraints,
			}
			outSchema.Properties = append(outSchema.Properties, prop)
			if len(pSchema.AdditionalTypes) > 0 {
				outSchema.AdditionalTypes = append(outSchema.AdditionalTypes, pSchema.AdditionalTypes...)
			}
		}

		descrs := [][]*base.SchemaProxy{schema.AnyOf, schema.OneOf}
		for _, descrItems := range descrs {
			nonNilDescrItems := make([]*base.SchemaProxy, 0)
			for _, item := range descrItems {
				if item != nil {
					t := item.Schema().Type
					if len(t) == 1 && t[0] == "null" {
						continue
					}
					nonNilDescrItems = append(nonNilDescrItems, item)
				}
			}
			if len(nonNilDescrItems) == 0 {
				continue
			}
			if len(nonNilDescrItems) == 1 {
				res, err := GenerateGoSchema(nonNilDescrItems[0], ref, path)
				if err != nil {
					return GoSchema{}, fmt.Errorf("error generating single type for anyOf: %w", err)
				}
				return res, nil
			} else {
				res, err := generateUnion(descrItems, schema.Discriminator, path)
				if err != nil {
					return GoSchema{}, fmt.Errorf("error generating type for anyOf: %w", err)
				}
				if res.Discriminator != nil {
					outSchema.Discriminator = res.Discriminator
				}
				if len(res.UnionElements) != 0 {
					outSchema.UnionElements = append(outSchema.UnionElements, res.UnionElements...)
				}
				outSchema.AdditionalTypes = append(outSchema.AdditionalTypes, res.AdditionalTypes...)
			}
		}

		fields := genFieldsFromProperties(outSchema.Properties)
		outSchema.GoType = outSchema.createGoStruct(fields)
	}

	// Check for x-go-type-name. It behaves much like x-go-type, however, it will
	// create a type definition for the named type, and use the named type in place
	// of this schema.
	if extension, ok := schemaExtensions.Get(extGoTypeName); ok {
		typeName, err := extString(extension.Value)
		if err != nil {
			return outSchema, fmt.Errorf("invalid value for %q: %w", extGoTypeName, err)
		}

		newTypeDef := TypeDefinition{
			Name:   typeName,
			Schema: outSchema,
		}
		outSchema = GoSchema{
			Description:     newTypeDef.Schema.Description,
			GoType:          typeName,
			DefineViaAlias:  true,
			AdditionalTypes: append(outSchema.AdditionalTypes, newTypeDef),
		}
	}

	return outSchema, nil
}
