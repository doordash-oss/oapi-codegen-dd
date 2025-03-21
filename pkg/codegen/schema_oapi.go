package codegen

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// oapiSchemaToGoType converts an OpenApi schema into a Go type definition for
// all non-object types.
func oapiSchemaToGoType(schema *openapi3.Schema, path []string) (Schema, error) {
	f := schema.Format
	t := schema.Type

	if t.Is("array") {
		// For arrays, we'll get the type of the Items and throw a
		// [] in front of it.
		arrayType, err := GenerateGoSchema(schema.Items, path)
		if err != nil {
			return Schema{}, fmt.Errorf("error generating type for array: %w", err)
		}
		if (arrayType.HasAdditionalProperties || len(arrayType.UnionElements) != 0) && arrayType.RefType == "" {
			// If we have items which have additional properties or union values,
			// but are not a pre-defined type, we need to define a type
			// for them, which will be based on the field names we followed
			// to get to the type.
			typeName := PathToTypeName(append(path, "Item"))

			typeDef := TypeDefinition{
				TypeName: typeName,
				JsonName: strings.Join(append(path, "Item"), "."),
				Schema:   arrayType,
			}
			arrayType.AdditionalTypes = append(arrayType.AdditionalTypes, typeDef)

			arrayType.RefType = typeName
		}
		outSchema := Schema{}
		outSchema.ArrayType = &arrayType
		outSchema.GoType = "[]" + arrayType.TypeDecl()
		outSchema.AdditionalTypes = arrayType.AdditionalTypes
		outSchema.Properties = arrayType.Properties
		outSchema.DefineViaAlias = true
		if disableTypeAliasesForArray {
			outSchema.DefineViaAlias = false
		}
		return outSchema, nil
	}

	if t.Is("integer") {
		outSchema := Schema{}
		// We default to int if format doesn't ask for something else.
		if f == "int64" {
			outSchema.GoType = "int64"
		} else if f == "int32" {
			outSchema.GoType = "int32"
		} else if f == "int16" {
			outSchema.GoType = "int16"
		} else if f == "int8" {
			outSchema.GoType = "int8"
		} else if f == "int" {
			outSchema.GoType = "int"
		} else if f == "uint64" {
			outSchema.GoType = "uint64"
		} else if f == "uint32" {
			outSchema.GoType = "uint32"
		} else if f == "uint16" {
			outSchema.GoType = "uint16"
		} else if f == "uint8" {
			outSchema.GoType = "uint8"
		} else if f == "uint" {
			outSchema.GoType = "uint"
		} else {
			outSchema.GoType = "int"
		}
		outSchema.DefineViaAlias = true
		return outSchema, nil
	}

	if t.Is("number") {
		outSchema := Schema{}
		// We default to float for "number"
		if f == "double" {
			outSchema.GoType = "float64"
		} else if f == "float" || f == "" {
			outSchema.GoType = "float32"
		} else {
			return Schema{}, fmt.Errorf("invalid number format: %s", f)
		}
		outSchema.DefineViaAlias = true
		return outSchema, nil
	}

	if t.Is("boolean") {
		if f != "" {
			return Schema{}, fmt.Errorf("invalid format (%s) for boolean", f)
		}
		outSchema := Schema{}
		outSchema.GoType = "bool"
		outSchema.DefineViaAlias = true
		return outSchema, nil
	}

	if t.Is("string") {
		outSchema := Schema{}
		// Special case string formats here.
		switch f {
		case "byte":
			outSchema.GoType = "[]byte"
		case "email":
			outSchema.GoType = "openapi_types.Email"
		case "date":
			outSchema.GoType = "openapi_types.Date"
		case "date-time":
			outSchema.GoType = "time.Time"
		case "json":
			outSchema.GoType = "json.RawMessage"
			outSchema.SkipOptionalPointer = true
		case "uuid":
			outSchema.GoType = "openapi_types.UUID"
		case "binary":
			outSchema.GoType = "openapi_types.File"
		default:
			// All unrecognized formats are simply a regular string.
			outSchema.GoType = "string"
		}
		outSchema.DefineViaAlias = true
		return outSchema, nil
	}

	return Schema{}, fmt.Errorf("unhandled Schema type: %v", t)
}
