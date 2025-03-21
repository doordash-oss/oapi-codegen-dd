package codegen

import (
	"errors"
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// UnionElement describe union element, based on prefix externalRef\d+ and real ref name from external schema.
type UnionElement string

// String returns externalRef\d+ and real ref name from external schema, like externalRef0.SomeType.
func (u UnionElement) String() string {
	return string(u)
}

// Method generate union method name for template functions `As/From/Merge`.
func (u UnionElement) Method() string {
	var method string
	for _, part := range strings.Split(string(u), `.`) {
		method += UppercaseFirstCharacter(part)
	}
	return method
}

func generateUnion(outSchema *Schema, elements openapi3.SchemaRefs, discriminator *openapi3.Discriminator, path []string) error {
	if discriminator != nil {
		outSchema.Discriminator = &Discriminator{
			Property: discriminator.PropertyName,
			Mapping:  make(map[string]string),
		}
	}

	refToGoTypeMap := make(map[string]string)
	for i, element := range elements {
		elementPath := append(path, fmt.Sprint(i))
		elementSchema, err := GenerateGoSchema(element, elementPath)
		if err != nil {
			return err
		}

		if element.Ref == "" {
			elementName := SchemaNameToTypeName(PathToTypeName(elementPath))
			if elementSchema.TypeDecl() == elementName {
				elementSchema.GoType = elementName
			} else {
				td := TypeDefinition{Schema: elementSchema, TypeName: elementName}
				outSchema.AdditionalTypes = append(outSchema.AdditionalTypes, td)
				elementSchema.GoType = td.TypeName
			}
			outSchema.AdditionalTypes = append(outSchema.AdditionalTypes, elementSchema.AdditionalTypes...)
		} else {
			refToGoTypeMap[element.Ref] = elementSchema.GoType
		}

		if discriminator != nil {
			if len(discriminator.Mapping) != 0 && element.Ref == "" {
				return errors.New("ambiguous discriminator.mapping: please replace inlined object with $ref")
			}

			// Explicit mapping.
			var mapped bool
			for k, v := range discriminator.Mapping {
				if v == element.Ref {
					outSchema.Discriminator.Mapping[k] = elementSchema.GoType
					mapped = true
					break
				}
			}
			// Implicit mapping.
			if !mapped {
				outSchema.Discriminator.Mapping[RefPathToObjName(element.Ref)] = elementSchema.GoType
			}
		}
		outSchema.UnionElements = append(outSchema.UnionElements, UnionElement(elementSchema.GoType))
	}

	if (outSchema.Discriminator != nil) && len(outSchema.Discriminator.Mapping) != len(elements) {
		return errors.New("discriminator: not all schemas were mapped")
	}

	return nil
}
