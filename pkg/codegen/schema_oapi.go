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
	"log/slog"
	"slices"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
)

// goPrimitiveTypes is a set of Go primitive type names.
// This is used to determine whether a type needs special handling (e.g., type definitions in unions).
var goPrimitiveTypes = map[string]bool{
	"string":    true,
	"int":       true,
	"int8":      true,
	"int16":     true,
	"int32":     true,
	"int64":     true,
	"uint":      true,
	"uint8":     true,
	"uint16":    true,
	"uint32":    true,
	"uint64":    true,
	"float":     true,
	"float32":   true,
	"float64":   true,
	"bool":      true,
	"time.Time": true,
	"struct{}":  true, // Empty struct - used for empty schemas
}

// isPrimitiveType returns true if the given type string is a Go primitive type.
func isPrimitiveType(typeDef string) bool {
	return goPrimitiveTypes[typeDef]
}

// oapiSchemaToGoType converts an OpenApi schema into a Go type definition for
// all non-object types.
func oapiSchemaToGoType(schema *base.Schema, options ParseOptions) (GoSchema, error) {
	f := schema.Format
	t := schema.Type

	path := options.path

	constraints := newConstraints(schema, ConstraintsContext{
		hasNilType:   slices.Contains(t, "null"),
		specLocation: options.specLocation,
	})

	// Handle multi-type schemas (union types like ["string", "number"]).
	// This is OpenAPI 3.1 syntax for type unions.
	// Count non-null types to determine if this is a union.
	var nonNullTypes []string
	for _, typ := range t {
		if typ != "null" {
			nonNullTypes = append(nonNullTypes, typ)
		}
	}
	if len(nonNullTypes) > 1 {
		// Multiple non-null types means this is a union type.
		// Build union elements from the primitive types.
		slog.Debug("multi-type schema (union)", "types", t, "path", path)
		return generateUnionFromTypes(nonNullTypes, schema, constraints, options)
	}

	if slices.Contains(t, "array") {
		// For arrays, we'll get the type of the Items and throw a
		// [] in front of it.
		opts := options
		var items *base.SchemaProxy
		var itemRef string
		if schema.Items != nil && schema.Items.IsA() {
			items = schema.Items.A
			itemRef = items.GoLow().GetReference()
			opts = opts.WithReference(itemRef)
			// For inline items (no reference), we don't append to the path here.
			// The path will be used for naming if needed (e.g., for unions or complex types).
			// We rely on the tracking logic to only track schemas with references or single-element paths.
		}

		// Pre-register the type name with the reference if available.
		// This allows circular references to find this type during processing.
		// We'll update the registration with the full type definition after processing.
		// Only pre-register if the ref is not already registered (to avoid overwriting).
		var preRegisteredTypeName string
		var weRegisteredRef bool // Track if WE registered the ref (vs it was already registered)
		if itemRef != "" && !isStandardComponentReference(itemRef) {
			if existingName, found := options.typeTracker.LookupByRef(itemRef); found {
				// The ref is already registered, use the existing name
				preRegisteredTypeName = existingName
				weRegisteredRef = false
			} else {
				preRegisteredTypeName = pathToTypeName(append(path, "Item"))
				if options.typeTracker.Exists(preRegisteredTypeName) {
					preRegisteredTypeName = options.typeTracker.generateUniqueName(preRegisteredTypeName)
				}
				// Pre-register with just the name so circular references can find it
				options.typeTracker.registerRef(itemRef, preRegisteredTypeName)
				weRegisteredRef = true
			}
		}

		arrayType, err := GenerateGoSchema(items, opts)
		if err != nil {
			return GoSchema{}, fmt.Errorf("error generating type for array: %w", err)
		}

		// Create a named type for array items if they have complex structure (additional properties,
		// union values, or properties) and are not already a named type or an array type.
		// Skip if items are an array - nested arrays should not create intermediate _Item types.
		// Also skip if the ref was already registered before we started (circular reference case).
		// If WE registered the ref (weRegisteredRef=true), we should still create the type.
		isArrayItems := arrayType.ArrayType != nil
		shouldCreateType := (arrayType.HasAdditionalProperties || len(arrayType.UnionElements) != 0 || len(arrayType.Properties) > 0) && arrayType.RefType == "" && !isArrayItems
		// If the ref was already registered before we started, skip creating the type
		// (this is a circular reference and the type will be created by the original caller)
		if shouldCreateType && itemRef != "" && !weRegisteredRef {
			shouldCreateType = false
		}
		if shouldCreateType {
			// If we have items which have additional properties, union values, or properties,
			// but are not a pre-defined type, we need to define a type
			// for them, which will be based on the field names we followed
			// to get to the type.
			// Use the pre-registered type name if available, otherwise generate a new one.
			typeName := preRegisteredTypeName
			if typeName == "" {
				typeName = pathToTypeName(append(path, "Item"))
				// Check if the type name already exists.
				// If it does, generate a unique name to avoid conflicts and overwrites.
				// This handles cases like allOf with duplicate property names where each
				// property has different enum values - we need separate types for each.
				if options.typeTracker.Exists(typeName) {
					typeName = options.typeTracker.generateUniqueName(typeName)
				}
			}

			typeDef := TypeDefinition{
				Name:             typeName,
				JsonName:         strings.Join(append(path, "Item"), "."),
				Schema:           arrayType,
				SpecLocation:     SpecLocationSchema,
				NeedsMarshaler:   needsMarshaler(arrayType),
				HasSensitiveData: hasSensitiveData(arrayType),
			}
			// Register the full type definition (the ref mapping was already done in pre-registration)
			options.typeTracker.register(typeDef, itemRef)
			arrayType.AdditionalTypes = append(arrayType.AdditionalTypes, typeDef)
			arrayType.RefType = typeName
		}

		// Determine the element type for the array.
		// If the items created a named type (RefType is set), use that.
		// Otherwise use GoType. This ensures nested arrays like [][]struct{}
		// become [][]TypeName when an _Item type was created.
		elemType := arrayType.RefType
		if elemType == "" {
			elemType = arrayType.GoType
		}

		// Add pointer prefix if the array items are explicitly nullable
		if schemaValueIsPointer(&arrayType) {
			elemType = "*" + strings.TrimPrefix(elemType, "*")
		}

		return GoSchema{
			GoType:          "[]" + elemType,
			ArrayType:       &arrayType,
			AdditionalTypes: arrayType.AdditionalTypes,
			Properties:      arrayType.Properties,
			Description:     schema.Description,
			OpenAPISchema:   schema,
			Constraints:     constraints,
		}, nil
	}

	goType := options.DefaultIntType
	if goType == "" {
		goType = "int"
	}

	if slices.Contains(t, "integer") {
		switch f {
		case "int64":
			goType = "int64"
		case "int32":
			goType = "int32"
		case "int16":
			goType = "int16"
		case "int8":
			goType = "int8"
		case "uint64":
			goType = "uint64"
		case "uint32":
			goType = "uint32"
		case "uint16":
			goType = "uint16"
		case "uint8":
			goType = "uint8"
		case "uint":
			goType = "uint"
		}

		return GoSchema{
			GoType:         goType,
			DefineViaAlias: true,
			Description:    schema.Description,
			OpenAPISchema:  schema,
			Constraints:    constraints,
		}, nil
	}

	if slices.Contains(t, "number") {
		// We default to float for "number"
		// Some specs incorrectly use type: number with format: integer
		// We handle this for compatibility with lenient validators
		switch f {
		case "double":
			goType = "float64"
		case "float":
			goType = "float32"
		case "decimal":
			// Non-standard format used by some specs to indicate arbitrary precision decimal
			// Treat as float64 for compatibility
			goType = "float64"
		case "integer", "int":
			// Treat type: number, format: integer or format: int as integer type
			// format: int is non-standard but used by some specs
			goType = options.DefaultIntType
			if goType == "" {
				goType = "int"
			}
		case "int32", "int64":
			// Also handle int32/int64 formats on number type
			goType = f
		case "":
			goType = "float32"
		default:
			// For unrecognized formats, default to float32 for compatibility
			// This handles invalid formats like "integer 0-100" gracefully
			goType = "float32"
		}

		return GoSchema{
			GoType:         goType,
			DefineViaAlias: true,
			Description:    schema.Description,
			OpenAPISchema:  schema,
			Constraints:    constraints,
		}, nil
	}

	if slices.Contains(t, "boolean") || slices.Contains(t, "bool") {
		// Ignore format for boolean types - OpenAPI spec doesn't define any valid formats.
		// Some specs incorrectly specify format for booleans, so we ignore it for compatibility.
		return GoSchema{
			GoType:         "bool",
			DefineViaAlias: true,
			Description:    schema.Description,
			OpenAPISchema:  schema,
			Constraints:    constraints,
		}, nil
	}

	if slices.Contains(t, "string") {
		// Special case string formats here.
		// All unrecognized formats are simply a regular string.
		goType := "string"
		skipOptionalPointer := false
		switch f {
		case "byte":
			goType = "[]byte"
		case "email":
			goType = "runtime.Email"
		case "date":
			goType = "runtime.Date"
		case "date-time":
			// If the schema has enum values, treat it as a string instead of time.Time
			// because enum values are string literals and time.Time cannot be used as constants
			if len(schema.Enum) > 0 {
				goType = "string"
			} else {
				goType = "time.Time"
			}
		case "json":
			goType = "json.RawMessage"
			skipOptionalPointer = true
		case "binary":
			goType = "runtime.File"
		case "uuid":
			// Only use uuid.UUID for standard UUID lengths (32 or 36 chars)
			// Non-standard lengths should use string to avoid unmarshal errors
			if isStandardUUIDLength(schema) {
				goType = "uuid.UUID"
			}
		}

		// Always use alias for primitives - validation will be handled
		// at the object level using validation tags
		return GoSchema{
			GoType:              goType,
			DefineViaAlias:      true,
			SkipOptionalPointer: skipOptionalPointer,
			Description:         schema.Description,
			OpenAPISchema:       schema,
			Constraints:         constraints,
		}, nil
	}

	if slices.Contains(t, "null") {
		return GoSchema{}, nil
	}

	// Handle unknown types gracefully by treating them as 'any'.
	// This allows code generation to proceed for specs with non-standard types
	// (e.g., "Timespan", "Guid", etc.) that are not valid OpenAPI types.
	// The generated code will compile and work at runtime, though type safety is reduced.
	if len(t) > 0 {
		slog.Debug("unknown OpenAPI type, treating as 'any'", "type", t)
		return GoSchema{
			GoType:         "any",
			DefineViaAlias: true,
			Description:    schema.Description,
			OpenAPISchema:  schema,
			Constraints:    constraints,
		}, nil
	}

	return GoSchema{}, fmt.Errorf("unhandled GoSchema type: %v", t)
}

// isStandardUUIDLength checks if the schema's length constraints are compatible with uuid.UUID.
// Standard UUID is 36 chars (with dashes) or 32 chars (without dashes).
// If no length constraints are specified, assumes standard UUID.
// Returns false for non-standard lengths (e.g., 64 chars) which should use string type.
func isStandardUUIDLength(schema *base.Schema) bool {
	// If no length constraints, assume standard UUID
	if schema.MinLength == nil && schema.MaxLength == nil {
		return true
	}

	// Check minLength
	if schema.MinLength != nil {
		minLen := *schema.MinLength
		if minLen != 32 && minLen != 36 {
			return false
		}
	}

	// Check maxLength
	if schema.MaxLength != nil {
		maxLen := *schema.MaxLength
		if maxLen != 32 && maxLen != 36 {
			return false
		}
	}

	return true
}
