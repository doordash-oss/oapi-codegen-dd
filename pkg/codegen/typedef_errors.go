// Copyright 2026 DoorDash, Inc.
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
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
)

// unknownErrorReturn is the Error() statement used when a mapped value is absent at runtime.
const unknownErrorReturn = `return "unknown error"`

var (
	errNoErrorMappingPath    = errors.New("no error-mapping path")
	errUnionVariantUnknown   = errors.New("its oneOf/anyOf union has more than two variants and no discriminator, so the decoded variant is unknown")
	errNoUnionVariantHasPath = errors.New("neither does any variant of its oneOf/anyOf union")
)

// errorPathSegment represents a parsed segment of an error mapping path.
type errorPathSegment struct {
	propertyName string
	isArrayIndex bool
}

// resolvedField contains all info needed for both Get and Set error response methods.
type resolvedField struct {
	goName        string
	goType        string
	containerType string // The type of the struct that contains this field (for nested struct literals)
	isNullable    bool
	isArray       bool
	arrayType     string
	isArrayIndex  bool
	prop          Property
	union         *resolvedUnion // Set on an embedded union field; the rest of the path continues in its variants
}

// resolvedUnion holds the variants of an embedded union that contain the rest of an error-mapping path.
type resolvedUnion struct {
	discriminated bool // The decoded variant comes from ValueByDiscriminator() instead of Value()
	variants      []resolvedVariant
}

// resolvedVariant is the rest of an error-mapping path inside one union variant.
type resolvedVariant struct {
	typeName string
	fields   []resolvedField
}

// applicableErrorMapping drops, with a warning, entries that cannot produce an Error() body, so no template calls a constructor that was never generated.
func applicableErrorMapping(mapping map[string]string, ctx *ParseContext) map[string]string {
	if len(mapping) == 0 || ctx == nil || ctx.TypeTracker == nil {
		return mapping
	}

	schemas := buildTypeSchemaMap(ctx)
	res := make(map[string]string, len(mapping))
	for _, name := range slices.Sorted(maps.Keys(mapping)) {
		path := mapping[name]
		if !ctx.TypeTracker.NeedsErrorMethod(name) {
			args := []any{"type", name, "path", path}
			if target := ctx.TypeTracker.resolveAliasChain(name); target != name && ctx.TypeTracker.NeedsErrorMethod(target) {
				args = append(args, "hint", fmt.Sprintf("%s is an alias of %s, map %s instead", name, target, target))
			}
			slog.Warn("error-mapping: ignoring entry, no error response type has this name", args...)
			continue
		}
		if _, err := resolveErrorPath(name, mapping, schemas[name], schemas); err != nil {
			slog.Warn(`error-mapping: ignoring entry whose path does not resolve, the type's Error() returns "unmapped client error"`,
				"type", name, "path", path, "reason", err.Error())
			continue
		}
		res[name] = path
	}
	return res
}

// errorConstructors lists the mapped error types that get a New<Type>(message string) constructor.
func errorConstructors(mapping map[string]string, typeSchemaMap map[string]GoSchema) map[string]bool {
	res := make(map[string]bool, len(mapping))
	for name := range mapping {
		td := TypeDefinition{Name: name, Schema: typeSchemaMap[name]}
		if td.GetErrorConstructor(mapping, typeSchemaMap) != "" {
			res[name] = true
		}
	}
	return res
}

// resolveErrorPath traverses the schema following the error-mapping path and returns
// resolved field info for each segment. The error explains the first segment that cannot be resolved.
func resolveErrorPath(typeName string, errTypes map[string]string, schema GoSchema, typeSchemaMap map[string]GoSchema) ([]resolvedField, error) {
	path, ok := errTypes[typeName]
	if !ok || path == "" {
		return nil, errNoErrorMappingPath
	}

	segments := parseErrorPath(path)
	if len(segments) == 0 {
		return nil, errNoErrorMappingPath
	}

	return resolveErrorSegments(typeName, segments, schema, typeSchemaMap, map[string]bool{})
}

// parseErrorPath parses an error mapping path like "data[].message[]" into segments.
func parseErrorPath(path string) []errorPathSegment {
	parts := strings.Split(path, ".")
	segments := make([]errorPathSegment, 0, len(parts))

	for _, part := range parts {
		isArray := strings.HasSuffix(part, "[]")
		propName := strings.TrimSuffix(part, "[]")
		segments = append(segments, errorPathSegment{
			propertyName: propName,
			isArrayIndex: isArray,
		})
	}

	return segments
}

// resolveErrorSegments resolves segments against schema, continuing inside the variants of an
// embedded union when a segment is not a direct property. visited guards against recursive unions.
func resolveErrorSegments(typeName string, segments []errorPathSegment, schema GoSchema, typeSchemaMap map[string]GoSchema, visited map[string]bool) ([]resolvedField, error) {
	fields := make([]resolvedField, 0, len(segments))
	// Track the current container type (the type of the struct we're looking at)
	currentContainerType := typeName

	for i, seg := range segments {
		found := false
		for _, prop := range schema.Properties {
			if prop.JsonFieldName == seg.propertyName {
				isNullable := prop.Constraints.Nullable != nil && *prop.Constraints.Nullable
				isArray := prop.Schema.ArrayType != nil

				f := resolvedField{
					goName:        prop.GoName,
					goType:        referencedTypeName(prop.Schema),
					containerType: currentContainerType,
					isNullable:    isNullable,
					isArray:       isArray,
					isArrayIndex:  seg.isArrayIndex,
					prop:          prop,
				}

				if isArray && prop.Schema.ArrayType != nil {
					f.arrayType = prop.Schema.ArrayType.GoType
				}

				fields = append(fields, f)
				schema = prop.Schema

				// If array access, get the element schema
				if seg.isArrayIndex && schema.ArrayType != nil {
					schema = *schema.ArrayType
				}

				// If the property references another type, resolve it
				if ref := referencedTypeName(schema); ref != "" && len(schema.Properties) == 0 {
					if resolvedSchema, ok := typeSchemaMap[ref]; ok {
						// Update container type to the referenced type before resolving
						currentContainerType = ref
						schema = resolvedSchema
					}
				} else if schema.GoType != "" {
					// The schema has properties already resolved - use its GoType
					currentContainerType = schema.GoType
				}

				found = true
				break
			}
		}
		if found {
			continue
		}

		owner := typeName
		if i > 0 {
			owner = "'" + joinErrorPath(segments[:i]) + "'"
		}
		unionProp, union, ok := embeddedUnion(schema, typeSchemaMap)
		if !ok {
			return nil, fmt.Errorf("%s has no property '%s'", owner, seg.propertyName)
		}
		variants, err := resolveUnionVariants(unionProp.Schema.RefType, union, segments[i:], typeSchemaMap, visited)
		if err != nil {
			return nil, fmt.Errorf("%s has no property '%s', and %w", owner, seg.propertyName, err)
		}
		return append(fields, resolvedField{
			goName:        unionProp.GoName,
			goType:        unionProp.Schema.RefType,
			containerType: currentContainerType,
			isNullable:    unionProp.Constraints.Nullable != nil && *unionProp.Constraints.Nullable,
			prop:          unionProp,
			union:         variants,
		}), nil
	}

	return fields, nil
}

// resolveUnionVariants resolves segments inside each variant of union, keeping the variants that have the path.
func resolveUnionVariants(name string, union GoSchema, segments []errorPathSegment, typeSchemaMap map[string]GoSchema, visited map[string]bool) (*resolvedUnion, error) {
	// Only a two-variant Either or a discriminator tells which variant was decoded.
	discriminated := len(union.UnionElements) != 2
	if discriminated && (union.Discriminator == nil || len(union.Discriminator.Mapping) == 0) {
		return nil, errUnionVariantUnknown
	}

	// Entering the same union with the same remaining path again means a variant embeds it recursively.
	key := fmt.Sprintf("%s:%d", name, len(segments))
	if visited[key] {
		return nil, errNoUnionVariantHasPath
	}
	visited[key] = true
	defer delete(visited, key)

	res := &resolvedUnion{discriminated: discriminated}
	for _, element := range union.UnionElements {
		schema := element.Schema
		if resolved, ok := typeSchemaMap[element.TypeName]; ok {
			schema = resolved
		}
		if fields, err := resolveErrorSegments(element.TypeName, segments, schema, typeSchemaMap, visited); err == nil {
			res.variants = append(res.variants, resolvedVariant{typeName: element.TypeName, fields: fields})
		}
	}
	if len(res.variants) == 0 {
		return nil, errNoUnionVariantHasPath
	}
	return res, nil
}

// embeddedUnion returns the first oneOf/anyOf union field embedded in schema.
func embeddedUnion(schema GoSchema, typeSchemaMap map[string]GoSchema) (Property, GoSchema, bool) {
	for _, prop := range schema.Properties {
		if union, ok := typeSchemaMap[prop.Schema.RefType]; ok && prop.JsonFieldName == "" && len(union.UnionElements) > 0 {
			return prop, union, true
		}
	}
	return Property{}, GoSchema{}, false
}

// errorResponseCode emits the statements that walk fields from prevVar and return the message.
func errorResponseCode(fields []resolvedField, prevVar string, varIndex *int) []string {
	var code []string
	for _, entry := range fields {
		varName := fmt.Sprintf("res%d", *varIndex)
		code = append(code, fmt.Sprintf("%s := %s.%s", varName, prevVar, entry.goName))

		// For nullable non-array types, add nil check and dereference
		// For arrays, we handle nil check in the array access section (via len check)
		if entry.isNullable && !entry.isArray {
			code = append(code, fmt.Sprintf("if %s == nil { %s }", varName, unknownErrorReturn))

			// Prepare for next access with dereference
			*varIndex++
			derefVar := fmt.Sprintf("res%d", *varIndex)
			code = append(code, fmt.Sprintf("%s := *%s", derefVar, varName))
			prevVar = derefVar
		} else {
			prevVar = varName
		}

		*varIndex++

		// Handle array access
		if entry.isArrayIndex {
			code = append(code, fmt.Sprintf("if len(%s) == 0 { %s }", prevVar, unknownErrorReturn))

			varName = fmt.Sprintf("res%d", *varIndex)
			code = append(code, fmt.Sprintf("%s := %s[0]", varName, prevVar))
			prevVar = varName
			*varIndex++
		}

		if entry.union != nil {
			return append(code, unionErrorResponseCode(entry.union, prevVar, varIndex)...)
		}
	}

	return append(code, fmt.Sprintf("return %s", prevVar))
}

// unionErrorResponseCode switches on the decoded variant of the union in prevVar; variants without the path fall through.
func unionErrorResponseCode(union *resolvedUnion, prevVar string, varIndex *int) []string {
	var code []string
	value := prevVar + ".Value()"
	if union.discriminated {
		value = fmt.Sprintf("res%d", *varIndex)
		*varIndex++
		code = append(code, fmt.Sprintf("%s, _ := %s.ValueByDiscriminator()", value, prevVar))
	}

	variantVar := fmt.Sprintf("res%d", *varIndex)
	*varIndex++
	code = append(code, fmt.Sprintf("switch %s := %s.(type) {", variantVar, value))
	for _, variant := range union.variants {
		code = append(code, fmt.Sprintf("case %s:", variant.typeName))
		code = append(code, errorResponseCode(variant.fields, variantVar, varIndex)...)
	}
	return append(code, "}", unknownErrorReturn)
}

// errorConstructorCode builds the New<Type>(message string) constructor for a path without union hops.
func errorConstructorCode(typeName string, fields []resolvedField) string {
	// Build nested struct literal from inside out
	// Start with the function parameter "message" and wrap with struct literals going outward
	innerExpr := "message"

	// Process fields in reverse order to build from inside out
	for i := len(fields) - 1; i >= 0; i-- {
		f := fields[i]

		// Get the next field's goName (the field we're assigning to in the inner struct)
		var nextFieldName string
		if i < len(fields)-1 {
			nextFieldName = fields[i+1].goName
		}

		if f.isArrayIndex {
			// Array field - wrap in slice with single element
			if f.arrayType != "" {
				if nextFieldName != "" {
					innerExpr = fmt.Sprintf("[]%s{{%s: %s}}", f.arrayType, nextFieldName, innerExpr)
				} else {
					innerExpr = fmt.Sprintf("[]%s{%s}", f.arrayType, innerExpr)
				}
			} else {
				innerExpr = fmt.Sprintf("[]%s{%s}", f.goType, innerExpr)
			}
		} else if f.isNullable && !f.isArray {
			// Pointer field - need to take address of struct literal or use runtime.Ptr for primitives
			if nextFieldName != "" {
				innerExpr = fmt.Sprintf("&%s{%s: %s}", f.goType, nextFieldName, innerExpr)
			} else {
				// Last field is a nullable primitive - use runtime.Ptr
				innerExpr = fmt.Sprintf("runtime.Ptr(%s)", innerExpr)
			}
		} else if nextFieldName != "" {
			// Non-nullable, non-array struct field - wrap with struct literal
			innerExpr = fmt.Sprintf("%s{%s: %s}", f.goType, nextFieldName, innerExpr)
		}
		// For the last field (no nextFieldName), it's either:
		// - A nullable primitive already wrapped with runtime.Ptr() above
		// - A non-nullable primitive that stays as "message"
	}

	return fmt.Sprintf("func New%s(message string) %s {\n\treturn %s{%s: %s}\n}",
		typeName, typeName, typeName, fields[0].goName, innerExpr)
}

// referencedTypeName returns the type a schema names; inline union wrappers only carry a RefType.
func referencedTypeName(schema GoSchema) string {
	if schema.GoType != "" {
		return schema.GoType
	}
	return schema.RefType
}

// joinErrorPath renders segments back into their dotted error-mapping form.
func joinErrorPath(segments []errorPathSegment) string {
	parts := make([]string, len(segments))
	for i, seg := range segments {
		parts[i] = seg.propertyName
		if seg.isArrayIndex {
			parts[i] += "[]"
		}
	}
	return strings.Join(parts, ".")
}
