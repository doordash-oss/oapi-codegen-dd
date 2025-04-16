package codegen

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
)

// GoSchema describes an OpenAPI schema, with lots of helper fields to use in the templating engine.
// GoType is the Go type that represents the schema.
// RefType is the type name of the schema, if it has one.
// ArrayType is the schema of the array element, if it's an array.
// EnumValues is a map of enum values.
// Properties is a list of fields for an object.
// HasAdditionalProperties is true if the object has additional properties.
// AdditionalPropertiesType is the type of additional properties.
// AdditionalTypes is a list of auxiliary types that may be needed.
// SkipOptionalPointer is true if the type doesn't need a * in front when it's optional.
// Description is the description of the element.
// Constraints is a struct that holds constraints for the schema.
// UnionElements is a list of possible elements in a oneOf/anyOf union.
// Discriminator describes which value is stored in a union.
// DefineViaAlias is true if the schema should be declared via alias.
type GoSchema struct {
	GoType                   string
	RefType                  string
	ArrayType                *GoSchema
	EnumValues               map[string]string
	Properties               []Property
	HasAdditionalProperties  bool
	AdditionalPropertiesType *GoSchema
	AdditionalTypes          []TypeDefinition
	SkipOptionalPointer      bool
	Description              string
	Constraints              Constraints

	UnionElements []UnionElement
	Discriminator *Discriminator

	DefineViaAlias bool
	OpenAPISchema  *base.Schema
}

func (s GoSchema) IsRef() bool {
	return s.RefType != ""
}

func (s GoSchema) IsExternalRef() bool {
	if !s.IsRef() {
		return false
	}
	return strings.Contains(s.RefType, ".")
}

func (s GoSchema) TypeDecl() string {
	if s.IsRef() {
		return s.RefType
	}
	return s.GoType
}

func (s GoSchema) IsZero() bool {
	return s.TypeDecl() == ""
}

func (s GoSchema) GetAdditionalTypeDefs() []TypeDefinition {
	return s.AdditionalTypes
}

func (s GoSchema) createGoStruct(fields []string) string {
	// Start out with struct {
	objectParts := []string{"struct {"}

	// Append all the field definitions
	objectParts = append(objectParts, fields...)

	// Close the struct
	if s.HasAdditionalProperties {
		objectParts = append(objectParts,
			fmt.Sprintf("AdditionalProperties map[string]%s `json:\"-\"`",
				additionalPropertiesType(s)))
	}

	if len(s.UnionElements) != 0 {
		objectParts = append(objectParts, "union json.RawMessage")
	}

	objectParts = append(objectParts, "}")
	return strings.Join(objectParts, "\n")
}

type Discriminator struct {
	// maps discriminator value to go type
	Mapping map[string]string

	// JSON property name that holds the discriminator
	Property string
}

func (d *Discriminator) JSONTag() string {
	return fmt.Sprintf("`json:\"%s\"`", d.Property)
}

func (d *Discriminator) PropertyName() string {
	return schemaNameToTypeName(d.Property)
}

func GenerateGoSchema(schemaProxy *base.SchemaProxy, ref string, path []string, options ParseOptions) (GoSchema, error) {
	// Add a fallback value in case the schemaProxy is nil.
	// i.e. the parent schema defines a type:array, but the array has
	// no items defined. Therefore, we have at least valid Go-Code.
	if schemaProxy == nil {
		return GoSchema{GoType: "any"}, nil
	}

	schema := schemaProxy.Schema()

	// use the referenced type:
	// properties will be picked up from the referenced schema later.
	if ref != "" {
		refType, err := refPathToGoType(ref)
		if err != nil {
			return GoSchema{}, fmt.Errorf("error turning reference (%s) into a Go type: %s", schemaProxy.GetReference(), err)
		}
		return GoSchema{
			GoType:         refType,
			DefineViaAlias: true,
			Description:    schema.Description,
			OpenAPISchema:  schema,
		}, nil
	}

	outSchema := GoSchema{
		Description:   schema.Description,
		OpenAPISchema: schema,
	}

	if schema.AllOf != nil {
		mergedSchema, err := mergeSchemas(schema.AllOf, path, options)
		if err != nil {
			return GoSchema{}, fmt.Errorf("error merging schemas: %w", err)
		}
		mergedSchema.OpenAPISchema = schema
		return mergedSchema, nil
	}

	extensions := extractExtensions(schema.Extensions)
	// Check x-go-type, which will completely override the definition of this
	// schema with the provided type.
	if extension, ok := extensions[extPropGoType]; ok {
		typeName, err := parseString(extension)
		if err != nil {
			return outSchema, fmt.Errorf("invalid value for %q: %w", extPropGoType, err)
		}
		outSchema.GoType = typeName
		outSchema.DefineViaAlias = true

		return outSchema, nil
	}

	// Check x-go-type-skip-optional-pointer, which will override if the type
	// should be a pointer or not when the field is optional.
	if extension, ok := extensions[extPropGoTypeSkipOptionalPointer]; ok {
		skipOptionalPointer, err := parseBooleanValue(extension)
		if err != nil {
			return outSchema, fmt.Errorf("invalid value for %q: %w", extPropGoTypeSkipOptionalPointer, err)
		}
		outSchema.SkipOptionalPointer = skipOptionalPointer
	}

	// GoSchema type and format, eg. string / binary
	t := schema.Type
	// Handle objects and empty schemas first as a special case
	if t == nil || slices.Contains(t, "object") {
		return createObjectSchema(schema, ref, path, options)
	}

	if len(schema.Enum) > 0 {
		return createEnumsSchema(schema, ref, path, options)
	}

	outSchema, err := oapiSchemaToGoType(schema, ref, path, options)
	if err != nil {
		return GoSchema{}, fmt.Errorf("error resolving primitive type: %w", err)
	}
	return outSchema, nil
}

// SchemaDescriptor describes a GoSchema, a type definition.
type SchemaDescriptor struct {
	Fields                   []FieldDescriptor
	HasAdditionalProperties  bool
	AdditionalPropertiesType string
}

type FieldDescriptor struct {
	Required bool   // Is the schema required? If not, we'll pass by pointer
	GoType   string // The Go type needed to represent the json type.
	GoName   string // The Go compatible type name for the type
	JsonName string // The json type name for the type
	IsRef    bool   // Is this schema a reference to predefined object?
}

func additionalPropertiesType(schema GoSchema) string {
	addPropsType := schema.AdditionalPropertiesType.GoType
	if schema.AdditionalPropertiesType.RefType != "" {
		addPropsType = schema.AdditionalPropertiesType.RefType
	}
	if schema.AdditionalPropertiesType.OpenAPISchema != nil {
		nullablePtr := schema.AdditionalPropertiesType.OpenAPISchema.Nullable
		if nullablePtr != nil && *nullablePtr {
			addPropsType = "*" + addPropsType
		}
	}
	return addPropsType
}

func schemaHasAdditionalProperties(schema *base.Schema) bool {
	if schema == nil || schema.AdditionalProperties == nil {
		return false
	}

	if schema.AdditionalProperties.IsA() && schema.AdditionalProperties.A != nil {
		return true
	}

	if schema.AdditionalProperties.IsB() && schema.AdditionalProperties.B {
		return true
	}
	return false
}
