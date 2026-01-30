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
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
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
	// True if this schema is a struct wrapper around a union (embedded Either or union field)
	IsUnionWrapper bool

	DefineViaAlias   bool
	IsPrimitiveAlias bool
	OpenAPISchema    *base.Schema
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

// TypeDeclWithNullable returns the type declaration with a pointer prefix if the schema
// is explicitly nullable (via nullable: true or type includes "null").
// This is used for additionalProperties value types where we need to represent null values.
func (s GoSchema) TypeDeclWithNullable() string {
	typeDef := s.TypeDecl()

	// Check if the value type should be a pointer (nullable)
	if additionalPropertiesValueIsPointer(&s) {
		return "*" + strings.TrimPrefix(typeDef, "*")
	}

	return typeDef
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

// NeedsValidation returns true if this schema represents a type that might have a Validate() method.
// This includes:
// - Types with RefType set (references to other types, inline types)
// - Component references (GoType is set but not a primitive type and not a primitive alias)
// - Struct types (has properties)
// - Union types (has union elements)
// - Types with validation constraints
func (s GoSchema) NeedsValidation() bool {
	// External refs don't need validation (they're from other packages)
	if s.IsExternalRef() {
		return false
	}

	// If RefType is set, it's a reference to another type that might have Validate()
	if s.RefType != "" {
		return true
	}

	// If it's a primitive alias, it doesn't have Validate()
	if s.IsPrimitiveAlias {
		return false
	}

	// If it has validation tags, it needs validation
	if len(s.Constraints.ValidationTags) > 0 {
		return true
	}

	// If it has union elements, it needs validation
	if len(s.UnionElements) > 0 {
		return true
	}

	// If it has properties, check if any of them need validation
	if len(s.Properties) > 0 {
		for _, prop := range s.Properties {
			// Property has validation tags
			if len(prop.Constraints.ValidationTags) > 0 {
				return true
			}
			// Property needs custom validation (RefType, struct, union, etc.)
			if prop.needsCustomValidation() {
				return true
			}
		}
		// No properties need validation
		return false
	}

	// Check if it's a map with additionalProperties that need validation
	if s.AdditionalPropertiesType != nil {
		// Check if the map has minProperties/maxProperties constraints
		if s.Constraints.MinProperties != nil || s.Constraints.MaxProperties != nil {
			return true
		}
		// Check if the map value type needs validation
		if s.AdditionalPropertiesType.NeedsValidation() {
			return true
		}
		// No validation needed for this map
		return false
	}

	// Check if it's an array with items that need validation
	if s.ArrayType != nil {
		// Check if the array has minItems/maxItems constraints
		if s.Constraints.MinItems != nil || s.Constraints.MaxItems != nil {
			return true
		}
		// Check if the array item type needs validation
		if s.ArrayType.NeedsValidation() {
			return true
		}
		// No validation needed for this array
		return false
	}

	// Check if GoType is a component reference (not a primitive type)
	// Primitive types: string, int, int32, int64, float32, float64, bool, time.Time, byte
	// Also check for pointer types and array/map/struct types
	typeDecl := s.TypeDecl()
	if typeDecl == "" || typeDecl == "any" {
		return false
	}

	// Check if it's a primitive type
	if isPrimitiveType(typeDecl) || isPrimitiveType(strings.TrimPrefix(typeDecl, "*")) {
		return false
	}

	// If it starts with struct/map/[], it's handled elsewhere
	if strings.HasPrefix(typeDecl, "struct") || strings.HasPrefix(typeDecl, "map[") || strings.HasPrefix(typeDecl, "[]") {
		return false
	}

	// Otherwise, it's likely a component reference that might have Validate()
	return true
}

// ContainsUnions returns true if this schema or any of its nested schemas contain union types.
// This is used to determine if we can use simple validate.Struct() or need custom validation logic.
func (s GoSchema) ContainsUnions() bool {
	// Direct check: is this schema itself a union wrapper?
	if s.IsUnionWrapper {
		return true
	}

	// Check properties recursively
	for _, prop := range s.Properties {
		if prop.Schema.ContainsUnions() {
			return true
		}
	}

	// Check array items
	if s.ArrayType != nil && s.ArrayType.ContainsUnions() {
		return true
	}

	// Check map values
	if s.AdditionalPropertiesType != nil && s.AdditionalPropertiesType.ContainsUnions() {
		return true
	}

	// Note: We don't check RefTypes here because:
	// 1. If it's an internal ref, the referenced type will have its own Validate() method
	// 2. If it's an external ref, we can't know if it contains unions
	// 3. The validation will be delegated to the referenced type's Validate() method anyway

	return false
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

	// Resolve the schema, preferring the mutated model for component references
	schema := resolveSchema(schemaProxy, options.model)

	// Handle case where schema resolution returns nil (malformed specs with empty property definitions)
	if schema == nil {
		return GoSchema{GoType: "any"}, nil
	}

	ref := options.reference

	// Create a tracking key - prefer the schema's actual reference, then options.reference, then path
	var trackingKey string
	var schemaRef string
	if low := schemaProxy.GoLow(); low != nil {
		schemaRef = low.GetReference()
	}

	// Use the schema's actual reference if options.reference is not set AND we're processing
	// a nested property schema (like additionalProperties, array items, etc).
	// For top-level schemas (responses, request bodies, component schemas), respect the
	// options.reference even if it's empty, as it may be intentionally cleared.
	// Detect nested schemas by checking if the path contains known nested schema indicators.
	isNestedSchema := false
	for _, pathElement := range options.path {
		if pathElement == "AdditionalProperties" || pathElement == "Items" {
			isNestedSchema = true
			break
		}
	}
	if ref == "" && schemaRef != "" && isNestedSchema {
		ref = schemaRef
	}

	if schemaRef != "" {
		trackingKey = schemaRef
	} else if ref != "" {
		trackingKey = ref
	} else if len(options.path) > 0 {
		// For component schemas (single path element), construct the reference
		if len(options.path) == 1 {
			trackingKey = "#/components/schemas/" + options.path[0]
		} else {
			trackingKey = strings.Join(options.path, ".")
		}
	}

	// use the referenced type:
	// properties will be picked up from the referenced schema later.
	if ref != "" {
		// Check if this is a standard component reference (#/components/schemas/Foo)
		// vs a deep path reference (#/paths/.../properties/time)
		isComponentRef := isStandardComponentReference(ref)

		if isComponentRef {
			// For standard component references, just return the type name
			// The type definition already exists or will be created separately

			// Check for circular references before processing the reference
			if options.visited != nil && options.visited[trackingKey] {
				// We've encountered a circular reference
				// Return the referenced type name
				refType, err := refPathToGoType(ref)
				if err != nil {
					return GoSchema{}, fmt.Errorf("error turning reference (%s) into a Go type: %s", ref, err)
				}

				return GoSchema{
					GoType:         refType,
					DefineViaAlias: true,
					Description:    schema.Description,
					OpenAPISchema:  schema,
				}, nil
			}

			// Not a circular reference, just return the type name.
			// First, try to look up the actual type name from the type tracker by ref.
			// This is important because the type may have been renamed via x-go-name or due to conflicts.
			refType, err := refPathToGoType(ref)
			if err != nil {
				return GoSchema{}, fmt.Errorf("error turning reference (%s) into a Go type: %s", schemaProxy.GetReference(), err)
			}

			// Check if we have a type definition for this reference that might have a different name
			// (e.g., due to x-go-name extension or name conflicts)
			// Also check if the referenced type is a primitive alias
			isPrimitiveAlias := false
			if options.typeTracker != nil {
				// Try to find the type by its original reference path
				if actualName, found := options.typeTracker.LookupByRef(ref); found {
					refType = actualName
					// Check if this is a primitive type alias
					if td, exists := options.typeTracker.LookupByName(actualName); exists {
						if td.IsAlias() && isPrimitiveType(td.Schema.GoType) {
							isPrimitiveAlias = true
						}
					}
				}
			}

			// If we didn't find it in typeTracker (which might be empty during initial processing),
			// check the OpenAPI schema directly to see if it's a primitive type
			// BUT: Don't mark enum types as primitive aliases - they have custom Validate() methods
			if !isPrimitiveAlias && schema != nil && schema.Type != nil && len(schema.Type) > 0 {
				schemaType := schema.Type[0]
				// Primitive types: string, number, integer, boolean
				// But NOT if they have enum values (enums need custom validation)
				hasEnumValues := len(schema.Enum) > 0
				if !hasEnumValues && (schemaType == "string" || schemaType == "number" ||
					schemaType == "integer" || schemaType == "boolean") {
					isPrimitiveAlias = true
				}
			}

			// Check if we're in response context and the schema has writeOnly required fields.
			// If so, we need to generate an inline type instead of using the component reference,
			// because writeOnly fields should not be required in responses.
			needsInlineType := false
			if options.specLocation == SpecLocationResponse && schema != nil {
				needsInlineType = hasWriteOnlyRequiredFields(schema)
			}

			if !needsInlineType {
				// Return the schema with the resolved type name
				// Note: We don't set RefType for component references because:
				// 1. RefType affects TypeDecl() which would change the type name used in struct fields
				// 2. Component references already have their own type definitions with Validate() methods
				// 3. The validation will be delegated via the type assertion in Property.needsCustomValidation()
				// Include Constraints so that consumers (like connexions) can access min/max values
				// for data generation even when using component references.
				constraints := newConstraints(schema, ConstraintsContext{
					hasNilType:   slices.Contains(schema.Type, "null"),
					specLocation: options.specLocation,
				})
				return GoSchema{
					GoType:           refType,
					DefineViaAlias:   true,
					IsPrimitiveAlias: isPrimitiveAlias,
					Description:      schema.Description,
					OpenAPISchema:    schema,
					Constraints:      constraints,
				}, nil
			}

			// Fall through to generate an inline type for response schemas with writeOnly required fields
			// Clear the reference so the schema is processed as an inline type
			ref = ""
			options = options.WithReference("")
		}

		// For deep path references, we need to process the schema and create a type definition
		// Fall through to normal schema processing below
		// The ref will be used to generate the type name
	}

	// Check for circular references for non-reference schemas
	// Only track visited schemas that have an actual schema reference (schemaRef)
	// Don't track component schemas or inline schemas based on path alone
	// Note: ref may not be empty here if it's a deep path reference (not a component ref)
	shouldTrack := trackingKey != "" && schemaRef != ""

	if shouldTrack && options.visited != nil && options.visited[trackingKey] {
		// For non-reference circular dependencies, return the type name
		// This handles cases like recursive schemas (e.g., a Node with children of type Node)
		// First, try to look up the type by reference in the type tracker
		// This handles cases where the type was already created with a different name
		// (e.g., Report_ReportData_Item for #/components/schemas/Report/definitions/reportComponent)
		if options.typeTracker != nil && schemaRef != "" {
			if actualName, found := options.typeTracker.LookupByRef(schemaRef); found {
				return GoSchema{
					GoType:         actualName,
					DefineViaAlias: true,
					Description:    schema.Description,
					OpenAPISchema:  schema,
				}, nil
			}
		}
		// Fall back to generating a type name from the path
		typeName := pathToTypeName(options.path)
		return GoSchema{
			GoType:         typeName,
			DefineViaAlias: true,
			Description:    schema.Description,
			OpenAPISchema:  schema,
		}, nil
	}

	// Mark this schema as being visited to detect circular references
	// We'll unmark it when we're done processing to allow the same schema to be used elsewhere
	if shouldTrack {
		if options.visited == nil {
			options.visited = make(map[string]bool)
		}
		options.visited[trackingKey] = true
		// Defer unmarking to allow this schema to be processed again in a different context
		defer func() {
			delete(options.visited, trackingKey)
		}()
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
	// This handles cases like:
	// - allOf with a description-only schema and a $ref (DefineViaAlias=true)
	// - allOf with additionalProperties that results in a map type
	// - allOf with a single element that has a primitive type (string, int, bool, etc.)
	// - allOf with a single element that has an array type
	// - allOf with a single element that has enum values
	if !merged.IsZero() && (merged.DefineViaAlias || strings.HasPrefix(merged.GoType, "map[") ||
		isPrimitiveType(merged.GoType) || strings.HasPrefix(merged.GoType, "[]") || len(merged.EnumValues) > 0) {
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

	// If the schema has a component reference and the type already exists in the tracker,
	// return early with just the type reference. This prevents regenerating types
	// (like enums) that were already generated from component schemas.
	// This handles cases where options.reference was cleared (e.g., for inline responses)
	// but the schema itself is a ref to a component.
	// Only do this for inline schemas (path length > 1), not for component schemas/responses
	// (path length == 1) which need to create their own type definitions.
	if len(options.path) > 1 && schemaRef != "" && isStandardComponentReference(schemaRef) && options.typeTracker != nil {
		if actualName, found := options.typeTracker.LookupByRef(schemaRef); found {
			// The type already exists, just return a reference to it
			constraints := newConstraints(schema, ConstraintsContext{
				hasNilType:   slices.Contains(schema.Type, "null"),
				specLocation: options.specLocation,
			})
			return GoSchema{
				GoType:         actualName,
				DefineViaAlias: true,
				Description:    schema.Description,
				OpenAPISchema:  schema,
				Constraints:    constraints,
			}, nil
		}
	}

	// GoSchema type and format, eg. string / binary
	t := schema.Type

	// Handle const values without explicit type - infer type from the const value
	// This is common in OpenAPI 3.1 specs where discriminator properties use const
	// e.g., scope: { const: "organization" } should be treated as a string
	if t == nil && schema.Const != nil {
		// Infer type from const value - treat as string since const values are typically strings
		// in discriminator contexts
		constraints := newConstraints(schema, ConstraintsContext{
			specLocation: options.specLocation,
		})
		return GoSchema{
			GoType:         "string",
			DefineViaAlias: true,
			Description:    schema.Description,
			OpenAPISchema:  schema,
			Constraints:    constraints,
		}, nil
	}

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

	// Handle deep path references: create a type definition for the schema
	// This handles cases like #/paths/.../properties/time where the schema is inline
	// but referenced from multiple places
	if ref != "" && !isStandardComponentReference(ref) {
		// Generate a type name from the reference path
		refType, err := refPathToGoType(ref)
		if err != nil {
			return GoSchema{}, fmt.Errorf("error turning reference (%s) into a Go type: %w", ref, err)
		}

		// Create a type definition for this schema
		// Only create if we have a meaningful schema (not just a simple type alias)
		if len(enhanced.Properties) > 0 || enhanced.HasAdditionalProperties || len(enhanced.UnionElements) > 0 || !enhanced.DefineViaAlias {
			typeDef := TypeDefinition{
				Name:             refType,
				JsonName:         ref,
				Schema:           enhanced,
				SpecLocation:     options.specLocation,
				NeedsMarshaler:   needsMarshaler(enhanced),
				HasSensitiveData: hasSensitiveData(enhanced),
			}
			// Register with the reference so circular references can find this type
			options.typeTracker.register(typeDef, ref)
			enhanced.AdditionalTypes = append(enhanced.AdditionalTypes, typeDef)
			enhanced.RefType = refType
		} else {
			// For simple types (like string with format date), create a type alias
			typeDef := TypeDefinition{
				Name:           refType,
				JsonName:       ref,
				Schema:         enhanced,
				SpecLocation:   options.specLocation,
				NeedsMarshaler: false,
			}
			// Register with the reference so circular references can find this type
			options.typeTracker.register(typeDef, ref)
			enhanced.AdditionalTypes = append(enhanced.AdditionalTypes, typeDef)
			enhanced.RefType = refType
		}
	}

	return enhanced, nil
}

// resolveSchema resolves a SchemaProxy to its Schema, preferring the mutated model for component references.
// This ensures that when we have a $ref to a component schema, we get the mutated version from the model
// (which may have had properties filtered) instead of the stale low-level version.
func resolveSchema(schemaProxy *base.SchemaProxy, model *v3high.Document) *base.Schema {
	if schemaProxy == nil {
		return nil
	}

	// Check if this is a component schema reference
	if model != nil && model.Components != nil && model.Components.Schemas != nil {
		if low := schemaProxy.GoLow(); low != nil {
			ref := low.GetReference()
			if strings.HasPrefix(ref, "#/components/schemas/") {
				schemaName := strings.TrimPrefix(ref, "#/components/schemas/")
				if modelSchemaProxy, ok := model.Components.Schemas.Get(schemaName); ok && modelSchemaProxy != nil {
					// Use the schema from the model, which has been mutated
					return modelSchemaProxy.Schema()
				}
			}
		}
	}

	// Fall back to the default resolution
	return schemaProxy.Schema()
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

	// Check if the value type should be a pointer (nullable)
	// Similar logic to Property.IsPointerType() but for map values
	if additionalPropertiesValueIsPointer(schema.AdditionalPropertiesType) {
		addPropsType = "*" + strings.TrimPrefix(addPropsType, "*")
	}

	return addPropsType
}

// schemaValueIsPointer returns true if a schema's value type should be a pointer.
// This is used for additionalProperties values and array items where we need to
// represent null values explicitly. Similar to Property.IsPointerType() but for
// non-property contexts.
func schemaValueIsPointer(schema *GoSchema) bool {
	if schema == nil {
		return false
	}

	typeDef := schema.GoType
	if schema.RefType != "" {
		typeDef = schema.RefType
	}

	// Arrays and maps are already reference types, don't need pointers
	if strings.HasPrefix(typeDef, "[]") || strings.HasPrefix(typeDef, "map[") {
		return false
	}

	// Check if the OpenAPI schema explicitly has nullable: true or type includes "null"
	if schema.OpenAPISchema != nil {
		// Check for OpenAPI 3.1 style: type: ["string", "null"]
		if slices.Contains(schema.OpenAPISchema.Type, "null") {
			return true
		}

		// Check for OpenAPI 3.0 style: nullable: true
		if schema.OpenAPISchema.Nullable != nil && *schema.OpenAPISchema.Nullable {
			return true
		}
	}

	return false
}

// additionalPropertiesValueIsPointer is an alias for schemaValueIsPointer for backward compatibility.
func additionalPropertiesValueIsPointer(schema *GoSchema) bool {
	return schemaValueIsPointer(schema)
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
	if (len(src.Properties) == 0 && len(src.UnionElements) == 0) || src.RefType != "" {
		return src, ""
	}

	baseName := pathToTypeName(options.path)
	name := baseName

	if options.typeTracker.Exists(baseName) {
		name = options.typeTracker.generateUniqueName(baseName)
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

	specLocation := SpecLocationSchema
	if len(src.UnionElements) > 0 {
		specLocation = SpecLocationUnion
	}

	td := TypeDefinition{
		Name:           name,
		Schema:         src,
		SpecLocation:   specLocation,
		NeedsMarshaler: needsMarshal,
		JsonName:       "-",
	}
	options.typeTracker.register(td, "")

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
	// Only define via alias if we have a RefType but no properties or union elements
	// If we have properties or union elements, we need to generate a struct, not an alias
	// This ensures that methods like Error() can be generated for response types
	if other.RefType != "" && len(src.Properties) == 0 && len(src.UnionElements) == 0 {
		src.DefineViaAlias = true
	}

	return src
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

// isStandardComponentReference checks if a $ref is a standard component reference
// (e.g., #/components/schemas/Foo) vs a deep path reference
// (e.g., #/paths/.../properties/time)
func isStandardComponentReference(ref string) bool {
	parts := strings.Split(ref, "/")
	// Standard component references have exactly 4 parts: #, components, <type>, <name>
	// e.g., #/components/schemas/Foo
	// e.g., #/components/parameters/Bar
	// e.g., #/components/responses/Baz
	return len(parts) == 4 && parts[1] == "components"
}

// hasWriteOnlyRequiredFields checks if a schema has any writeOnly properties that are also required.
// This is used to determine if a response schema needs to generate an inline type instead of
// using a component reference, because writeOnly fields should not be required in responses.
func hasWriteOnlyRequiredFields(schema *base.Schema) bool {
	if schema == nil {
		return false
	}

	// Build a set of required field names
	requiredFields := make(map[string]bool)
	for _, req := range schema.Required {
		requiredFields[req] = true
	}

	// Check if any writeOnly property is also required
	if schema.Properties != nil {
		for propName, propProxy := range schema.Properties.FromOldest() {
			if propProxy == nil {
				continue
			}
			propSchema := propProxy.Schema()
			if propSchema != nil && propSchema.WriteOnly != nil && *propSchema.WriteOnly {
				if requiredFields[propName] {
					return true
				}
			}
		}
	}

	return false
}
