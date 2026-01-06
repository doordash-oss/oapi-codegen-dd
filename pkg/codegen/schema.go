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

// IsAnyType returns true if the schema represents the 'any' type or an array of 'any'.
// These types don't need validation methods since they accept any value.
func (s GoSchema) IsAnyType() bool {
	typeDecl := s.TypeDecl()
	return typeDecl == "any" || typeDecl == "[]any"
}

func (s GoSchema) GetAdditionalTypeDefs() []TypeDefinition {
	return s.AdditionalTypes
}

// ValidateDecl generates the body of the Validate() method for this schema.
// It returns the Go code that should appear inside the Validate() method.
// The alias parameter is the receiver variable name (e.g., "p" for "func (p Person) Validate()").
// The validatorVar parameter is the name of the validator variable to use (e.g., "bodyTypesValidate").
func (s GoSchema) ValidateDecl(alias string, validatorVar string) string {
	var lines []string

	// Handle array types
	if s.ArrayType != nil {
		hasConstraints := s.Constraints.MinItems != nil || s.Constraints.MaxItems != nil

		// If nullable and has constraints, check for nil first
		if s.Constraints.Nullable != nil && *s.Constraints.Nullable && hasConstraints {
			lines = append(lines, fmt.Sprintf("if %s == nil {", alias))
			lines = append(lines, "    return nil")
			lines = append(lines, "}")
		}

		// Check MinItems constraint
		if s.Constraints.MinItems != nil {
			lines = append(lines, fmt.Sprintf("if len(%s) < %d {", alias, *s.Constraints.MinItems))
			lines = append(lines, fmt.Sprintf("    return runtime.NewValidationError(\"\", fmt.Sprintf(\"must have at least %d items, got %%d\", len(%s)))", *s.Constraints.MinItems, alias))
			lines = append(lines, "}")
		}
		// Check MaxItems constraint
		if s.Constraints.MaxItems != nil {
			lines = append(lines, fmt.Sprintf("if len(%s) > %d {", alias, *s.Constraints.MaxItems))
			lines = append(lines, fmt.Sprintf("    return runtime.NewValidationError(\"\", fmt.Sprintf(\"must have at most %d items, got %%d\", len(%s)))", *s.Constraints.MaxItems, alias))
			lines = append(lines, "}")
		}
		// Validate array items if they have constraints or are custom types
		needsItemValidation := false
		if s.ArrayType.RefType != "" && !s.ArrayType.IsExternalRef() {
			needsItemValidation = true
		} else if len(s.ArrayType.Constraints.ValidationTags) > 0 {
			needsItemValidation = true
		}

		if needsItemValidation {
			lines = append(lines, "for i, item := range "+alias+" {")

			// If items have a Validate() method, call it
			if s.ArrayType.RefType != "" && !s.ArrayType.IsExternalRef() {
				lines = append(lines, "    if v, ok := any(item).(runtime.Validator); ok {")
				lines = append(lines, "        if err := v.Validate(); err != nil {")
				lines = append(lines, "            return runtime.NewValidationErrorFromError(fmt.Sprintf(\"[%d]\", i), err)")
				lines = append(lines, "        }")
				lines = append(lines, "    }")
			} else if len(s.ArrayType.Constraints.ValidationTags) > 0 {
				// Otherwise use validator tags
				tags := strings.Join(s.ArrayType.Constraints.ValidationTags, ",")
				lines = append(lines, fmt.Sprintf("    if err := %s.Var(item, \"%s\"); err != nil {", validatorVar, tags))
				lines = append(lines, "        return runtime.NewValidationErrorFromError(fmt.Sprintf(\"[%d]\", i), err)")
				lines = append(lines, "    }")
			}

			lines = append(lines, "}")
		}
		lines = append(lines, "return nil")
		return strings.Join(lines, "\n")
	}

	// If this schema has a RefType set, it means it's a reference to another type
	// In this case, we should delegate validation to the underlying type
	if s.RefType != "" && !s.IsExternalRef() {
		// Cast to the underlying type to avoid infinite recursion
		// (the current type might implement Validator itself)
		return fmt.Sprintf("if val, ok := any(%s(%s)).(runtime.Validator); ok {\n    return val.Validate()\n}\nreturn nil", s.RefType, alias)
	}

	// If this schema has properties but GoType is a reference to another type
	// (not a struct/map/slice), delegate to the underlying type
	typeDecl := s.TypeDecl()
	if len(s.Properties) > 0 && !strings.HasPrefix(typeDecl, "struct") &&
		!strings.HasPrefix(typeDecl, "map[") && !strings.HasPrefix(typeDecl, "[]") {
		// This is a type definition like "type X Y" where Y is another type
		// Cast to the underlying type to avoid infinite recursion
		return fmt.Sprintf("if val, ok := any(%s(%s)).(runtime.Validator); ok {\n    return val.Validate()\n}\nreturn nil", typeDecl, alias)
	}

	// Check if any property needs custom validation
	hasCustomValidation := false
	for _, prop := range s.Properties {
		if prop.needsCustomValidation() {
			hasCustomValidation = true
			break
		}
	}

	// If no custom validation needed
	if !hasCustomValidation {
		typeDecl := s.TypeDecl()

		// Handle map types (from additionalProperties)
		if strings.HasPrefix(typeDecl, "map[") {
			hasConstraints := s.Constraints.MinProperties != nil || s.Constraints.MaxProperties != nil

			// If nullable and has constraints, check for nil first
			if s.Constraints.Nullable != nil && *s.Constraints.Nullable && hasConstraints {
				lines = append(lines, fmt.Sprintf("if %s == nil {", alias))
				lines = append(lines, "    return nil")
				lines = append(lines, "}")
			}

			// Check MinProperties constraint
			if s.Constraints.MinProperties != nil {
				lines = append(lines, fmt.Sprintf("if len(%s) < %d {", alias, *s.Constraints.MinProperties))
				lines = append(lines, fmt.Sprintf("    return runtime.NewValidationError(\"\", fmt.Sprintf(\"must have at least %d properties, got %%d\", len(%s)))", *s.Constraints.MinProperties, alias))
				lines = append(lines, "}")
			}
			// Check MaxProperties constraint
			if s.Constraints.MaxProperties != nil {
				lines = append(lines, fmt.Sprintf("if len(%s) > %d {", alias, *s.Constraints.MaxProperties))
				lines = append(lines, fmt.Sprintf("    return runtime.NewValidationError(\"\", fmt.Sprintf(\"must have at most %d properties, got %%d\", len(%s)))", *s.Constraints.MaxProperties, alias))
				lines = append(lines, "}")
			}
			// Validate each value if it implements Validator
			if s.AdditionalPropertiesType != nil && s.AdditionalPropertiesType.RefType != "" && !s.AdditionalPropertiesType.IsExternalRef() {
				lines = append(lines, "for k, v := range "+alias+" {")
				lines = append(lines, "    if validator, ok := any(v).(runtime.Validator); ok {")
				lines = append(lines, "        if err := validator.Validate(); err != nil {")
				lines = append(lines, "            return runtime.NewValidationErrorFromError(k, err)")
				lines = append(lines, "        }")
				lines = append(lines, "    }")
				lines = append(lines, "}")
			}
			lines = append(lines, "return nil")
			return strings.Join(lines, "\n")
		}

		// For other non-struct types (slices, primitives), there's nothing to validate structurally
		if strings.HasPrefix(typeDecl, "[]") || len(s.Properties) == 0 {
			return "return nil"
		}

		// For struct types, use validator.Struct()
		return fmt.Sprintf("return %s.Struct(%s)", validatorVar, alias)
	}

	// Generate custom validation for each property
	for _, prop := range s.Properties {
		if prop.needsCustomValidation() {
			// Property needs custom validation - call Validate() method
			if prop.IsPointerType() {
				lines = append(lines, fmt.Sprintf("if %s.%s != nil {", alias, prop.GoName))
				lines = append(lines, fmt.Sprintf("    if v, ok := any(%s.%s).(runtime.Validator); ok {", alias, prop.GoName))
				lines = append(lines, "        if err := v.Validate(); err != nil {")
				lines = append(lines, fmt.Sprintf("            return runtime.NewValidationErrorFromError(\"%s\", err)", prop.GoName))
				lines = append(lines, "        }")
				lines = append(lines, "    }")
				lines = append(lines, "}")
			} else {
				lines = append(lines, fmt.Sprintf("if v, ok := any(%s.%s).(runtime.Validator); ok {", alias, prop.GoName))
				lines = append(lines, "    if err := v.Validate(); err != nil {")
				lines = append(lines, fmt.Sprintf("        return runtime.NewValidationErrorFromError(\"%s\", err)", prop.GoName))
				lines = append(lines, "    }")
				lines = append(lines, "}")
			}
		} else if len(prop.Constraints.ValidationTags) > 0 {
			// Property with validation tags - use Var()
			tags := strings.Join(prop.Constraints.ValidationTags, ",")
			if prop.IsPointerType() {
				lines = append(lines, fmt.Sprintf("if %s.%s != nil {", alias, prop.GoName))
				lines = append(lines, fmt.Sprintf("    if err := %s.Var(%s.%s, \"%s\"); err != nil {", validatorVar, alias, prop.GoName, tags))
				lines = append(lines, fmt.Sprintf("        return runtime.NewValidationErrorFromError(\"%s\", err)", prop.GoName))
				lines = append(lines, "    }")
				lines = append(lines, "}")
			} else {
				lines = append(lines, fmt.Sprintf("if err := %s.Var(%s.%s, \"%s\"); err != nil {", validatorVar, alias, prop.GoName, tags))
				lines = append(lines, fmt.Sprintf("        return runtime.NewValidationErrorFromError(\"%s\", err)", prop.GoName))
				lines = append(lines, "}")
			}
		}
	}

	lines = append(lines, "return nil")
	return strings.Join(lines, "\n")
}

func (s GoSchema) createGoStruct(fields []string) string {
	// Start out with struct {
	objectParts := []string{"struct {"}

	// Append all the field definitions
	objectParts = append(objectParts, fields...)

	// Close the struct
	if s.HasAdditionalProperties {
		objectParts = append(
			objectParts,
			fmt.Sprintf("AdditionalProperties map[string]%s `json:\"-\"`", additionalPropertiesType(s)),
		)
	}

	if len(s.UnionElements) == 2 {
		objectParts = append(objectParts, fmt.Sprintf("runtime.Either[%s, %s]", s.UnionElements[0], s.UnionElements[1]))
	} else if len(s.UnionElements) > 0 {
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

func GenerateGoSchema(schemaProxy *base.SchemaProxy, options ParseOptions) (GoSchema, error) {
	// Add a fallback value in case the schemaProxy is nil.
	// i.e. the parent schema defines a type:array, but the array has
	// no items defined. Therefore, we have at least valid Go-Code.
	if schemaProxy == nil {
		return GoSchema{GoType: "any"}, nil
	}

	schema := schemaProxy.Schema()

	ref := options.reference

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

	var (
		merged GoSchema
		err    error
	)

	merged, err = createFromCombinator(schema, options)
	if err != nil {
		return GoSchema{}, err
	}

	// If the combinator (allOf/anyOf/oneOf) resulted in a complete schema, return it directly
	// This handles cases like allOf with a description-only schema and a $ref
	if !merged.IsZero() && merged.DefineViaAlias {
		return merged, nil
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

		enhanced := enhanceSchema(outSchema, merged, options)
		return enhanced, nil
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
		res, err := createObjectSchema(schema, options)
		if err != nil {
			return GoSchema{}, err
		}

		enhanced := enhanceSchema(res, merged, options)
		return enhanced, nil
	}

	if len(schema.Enum) > 0 {
		res, err := createEnumsSchema(schema, options)
		if err != nil {
			return GoSchema{}, err
		}

		enhanced := enhanceSchema(res, merged, options)
		return enhanced, nil
	}

	outSchema, err = oapiSchemaToGoType(schema, options)
	if err != nil {
		return GoSchema{}, fmt.Errorf("error resolving primitive type: %w", err)
	}

	enhanced := enhanceSchema(outSchema, merged, options)
	return enhanced, nil
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

	return addPropsType
}

func schemaHasAdditionalProperties(schema *base.Schema) bool {
	if schema == nil || schema.AdditionalProperties == nil {
		return false
	}

	if schema.AdditionalProperties.IsA() {
		return true
	}

	if schema.AdditionalProperties.IsB() && schema.AdditionalProperties.B {
		return true
	}
	return false
}

func replaceInlineTypes(src GoSchema, options ParseOptions) (GoSchema, string) {
	if len(src.Properties) == 0 || src.RefType != "" {
		return src, ""
	}

	currentTypes := options.currentTypes
	baseName := options.baseName
	name := baseName
	if baseName == "" {
		baseName = pathToTypeName(options.path)
		name = baseName
	}

	if _, exists := currentTypes[baseName]; exists {
		name = generateTypeName(currentTypes, baseName, options.nameSuffixes)
	}

	isArrayType := src.ArrayType != nil

	// Calculate if this type needs a custom marshaler
	// For array type definitions like "type Foo []Bar", don't generate marshalers
	// Arrays handle marshaling automatically
	needsMarshal := needsMarshaler(src)
	if isArrayType {
		// This is an array type definition like "type Foo []Bar"
		// Don't generate marshalers - arrays handle marshaling automatically
		needsMarshal = false
	}

	td := TypeDefinition{
		Name:           name,
		Schema:         src,
		SpecLocation:   SpecLocationSchema,
		NeedsMarshaler: needsMarshal,
		JsonName:       "-",
	}
	options.AddType(td)

	return GoSchema{
		RefType:         name,
		AdditionalTypes: []TypeDefinition{td},
	}, name
}

func enhanceSchema(src, other GoSchema, options ParseOptions) GoSchema {
	if len(other.UnionElements) == 0 && len(other.Properties) == 0 {
		return src
	}

	src.Properties = append(src.Properties, other.Properties...)
	src.Discriminator = other.Discriminator
	src.UnionElements = other.UnionElements
	src.AdditionalTypes = append(src.AdditionalTypes, other.AdditionalTypes...)

	srcFields := genFieldsFromProperties(src.Properties, options)
	src.GoType = src.createGoStruct(srcFields)

	src.RefType = other.RefType
	if other.RefType != "" {
		src.DefineViaAlias = true
	}

	return src
}

func generateTypeName(currentTypes map[string]TypeDefinition, baseName string, suffixes []string) string {
	if currentTypes == nil {
		return baseName
	}
	if _, exists := currentTypes[baseName]; !exists {
		return baseName
	}

	if len(suffixes) == 0 {
		suffixes = []string{""}
	}

	for i := 0; ; i++ {
		for _, suffix := range suffixes {
			name := baseName + suffix
			if i > 0 {
				name = fmt.Sprintf("%s%d", name, i)
			}
			if _, exists := currentTypes[name]; !exists {
				return name
			}
		}
	}
}

func needsMarshaler(schema GoSchema) bool {
	// Check if any property has sensitive data
	if hasSensitiveData(schema) {
		return true
	}

	res := false
	for _, p := range schema.Properties {
		if p.JsonFieldName == "" {
			res = true
			break
		}
	}

	if !res {
		return false
	}

	// union types handled separately and they have marshaler.
	return len(schema.UnionElements) == 0
}

// hasSensitiveData checks if a schema has any properties marked as sensitive
func hasSensitiveData(schema GoSchema) bool {
	for _, p := range schema.Properties {
		if p.SensitiveData != nil {
			return true
		}
	}
	return false
}
