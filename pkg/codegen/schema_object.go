package codegen

import (
	"fmt"
	"slices"
	"strings"

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
			hasNilType: hasNilType,
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
		var err error

		outSchema, err = enhanceSchemaWithAdditionalProperties(outSchema, schema, options)
		if err != nil {
			return GoSchema{}, err
		}

		// If the schema has no properties, and only additional properties, we will
		// early-out here and generate a map[string]<schema> instead of an object
		// that contains this map. We skip over anyOf/oneOf here because they can
		// introduce properties. allOf was handled above.
		if (schema.Properties == nil || schema.Properties.Len() == 0) &&
			schema.AllOf == nil && schema.AnyOf == nil && schema.OneOf == nil {
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
			opts := options.WithReference(pRef).WithPath(propertyPath)
			pSchema, err := GenerateGoSchema(p, opts)
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
				typeName := pathToTypeName(append(propertyPath, "AdditionalProperties"))
				typeName = schemaNameToTypeName(typeName)

				var specLocation = SpecLocationSchema
				if len(pSchema.UnionElements) != 0 {
					specLocation = SpecLocationUnion
				}

				typeDef := TypeDefinition{
					Name:         typeName,
					JsonName:     strings.Join(propertyPath, "."),
					Schema:       pSchema,
					SpecLocation: specLocation,
				}
				options.AddType(typeDef)
				pSchema.AdditionalTypes = append(pSchema.AdditionalTypes, typeDef)
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

			pSchema = replaceInlineTypes(pSchema, opts)

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

		fields := genFieldsFromProperties(outSchema.Properties, options)
		outSchema.GoType = outSchema.createGoStruct(fields)
	}

	// Check for x-go-type-name. It behaves much like x-go-type, however, it will
	// create a type definition for the named type, and use the named type in place
	// of this schema.
	if extension, ok := schemaExtensions[extGoTypeName]; ok {
		typeName, err := parseString(extension)
		if err != nil {
			return outSchema, fmt.Errorf("invalid value for %q: %w", extGoTypeName, err)
		}

		newTypeDef := TypeDefinition{
			Name:   typeName,
			Schema: outSchema,
		}
		options.AddType(newTypeDef)
		outSchema = GoSchema{
			Description:     newTypeDef.Schema.Description,
			GoType:          typeName,
			DefineViaAlias:  true,
			AdditionalTypes: append(outSchema.AdditionalTypes, newTypeDef),
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
		additionalSchema, err := GenerateGoSchema(schema.AdditionalProperties.A, options)
		if err != nil {
			return GoSchema{}, fmt.Errorf("error generating type for additional properties: %w", err)
		}

		if len(additionalSchema.Properties) > 0 || additionalSchema.HasAdditionalProperties || len(additionalSchema.UnionElements) != 0 {
			// If we have fields present which have additional properties or union values,
			// but are not a pre-defined type, we need to define a type
			// for them, which will be based on the field names we followed
			// to get to the type.
			typeName := pathToTypeName(append(path, "AdditionalProperties"))
			typeName = schemaNameToTypeName(typeName)

			goTypeName, err := renameComponent(typeName, schema.ParentProxy)
			if err != nil {
				return GoSchema{}, fmt.Errorf("error making name for additional properties %s: %w", typeName, err)
			}

			typeDef := TypeDefinition{
				Name:         goTypeName,
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
