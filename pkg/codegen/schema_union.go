package codegen

import (
	"fmt"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
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

func generateUnion(elements []*base.SchemaProxy, discriminator *base.Discriminator, path []string, options ParseOptions) (GoSchema, error) {
	outSchema := GoSchema{}

	if discriminator != nil {
		outSchema.Discriminator = &Discriminator{
			Property: discriminator.PropertyName,
			Mapping:  make(map[string]string),
		}
	}

	refToGoTypeMap := make(map[string]string)

	for i, element := range elements {
		if element == nil {
			continue
		}
		elementPath := append(path, fmt.Sprint(i))
		ref := element.GoLow().GetReference()
		elementSchema, err := GenerateGoSchema(element, ref, elementPath, options)
		if err != nil {
			return GoSchema{}, err
		}

		if ref != "" {
			refToGoTypeMap[ref] = elementSchema.GoType
		}

		if ref == "" {
			elementName := schemaNameToTypeName(pathToTypeName(elementPath))

			if elementSchema.TypeDecl() == elementName {
				elementSchema.GoType = elementName
			} else {
				td := TypeDefinition{
					Schema:       elementSchema,
					Name:         elementName,
					SpecLocation: SpecLocationUnion,
				}
				outSchema.AdditionalTypes = append(outSchema.AdditionalTypes, td)
				elementSchema.GoType = td.Name
			}
			outSchema.AdditionalTypes = append(outSchema.AdditionalTypes, elementSchema.AdditionalTypes...)
		}

		if discriminator != nil {
			if discriminator.Mapping.Len() != 0 && element.GetReference() == "" {
				return GoSchema{}, ErrAmbiguousDiscriminatorMapping
			}

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
				outSchema.Discriminator.Mapping[refPathToObjName(element.GetReference())] = elementSchema.GoType
			}
		}
		outSchema.UnionElements = append(outSchema.UnionElements, UnionElement(elementSchema.GoType))
	}

	if (outSchema.Discriminator != nil) && len(outSchema.Discriminator.Mapping) != len(elements) {
		return GoSchema{}, ErrDiscriminatorNotAllMapped
	}

	return outSchema, nil
}
