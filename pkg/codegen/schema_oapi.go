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

// oapiSchemaToGoType converts an OpenApi schema into a Go type definition for
// all non-object types.
func oapiSchemaToGoType(schema *base.Schema, options ParseOptions) (GoSchema, error) {
	f := schema.Format
	t := schema.Type

	path := options.path

	constraints := newConstraints(schema, ConstraintsContext{
		name:         "",
		hasNilType:   slices.Contains(t, "null"),
		specLocation: options.specLocation,
	})

	if slices.Contains(t, "array") {
		// For arrays, we'll get the type of the Items and throw a
		// [] in front of it.
		opts := options
		var items *base.SchemaProxy
		if schema.Items != nil && schema.Items.IsA() {
			items = schema.Items.A
			ref := items.GoLow().GetReference()
			opts = opts.WithReference(ref)
			// For inline items (no reference), we don't append to the path here.
			// The path will be used for naming if needed (e.g., for unions or complex types).
			// We rely on the tracking logic to only track schemas with references or single-element paths.
		}

		arrayType, err := GenerateGoSchema(items, opts)
		if err != nil {
			return GoSchema{}, fmt.Errorf("error generating type for array: %w", err)
		}

		if (arrayType.HasAdditionalProperties || len(arrayType.UnionElements) != 0 || len(arrayType.Properties) > 0) && arrayType.RefType == "" {
			// If we have items which have additional properties, union values, or properties,
			// but are not a pre-defined type, we need to define a type
			// for them, which will be based on the field names we followed
			// to get to the type.
			typeName := pathToTypeName(append(path, "Item"))

			typeDef := TypeDefinition{
				Name:             typeName,
				JsonName:         strings.Join(append(path, "Item"), "."),
				Schema:           arrayType,
				SpecLocation:     SpecLocationSchema,
				NeedsMarshaler:   needsMarshaler(arrayType),
				HasSensitiveData: hasSensitiveData(arrayType),
			}
			options.AddType(typeDef)
			arrayType.AdditionalTypes = append(arrayType.AdditionalTypes, typeDef)
			arrayType.RefType = typeName
		}

		return GoSchema{
			GoType:          "[]" + arrayType.TypeDecl(),
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
			goType = "runtime.RawMessage"
			skipOptionalPointer = true
		case "binary":
			goType = "runtime.File"
		case "uuid":
			goType = "uuid.UUID"
		}

		// Don't use alias if we have length constraints that need validation
		// (we can't add methods to type aliases)
		defineViaAlias := true
		if goType == "string" && (constraints.MinLength != nil || constraints.MaxLength != nil) {
			defineViaAlias = false
		}

		return GoSchema{
			GoType:              goType,
			DefineViaAlias:      defineViaAlias,
			SkipOptionalPointer: skipOptionalPointer,
			Description:         schema.Description,
			OpenAPISchema:       schema,
			Constraints:         constraints,
		}, nil
	}

	if slices.Contains(t, "null") {
		return GoSchema{}, nil
	}

	return GoSchema{}, fmt.Errorf("unhandled GoSchema type: %v", t)
}
