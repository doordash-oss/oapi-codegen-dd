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
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
)

// This file contains all validation generation logic for GoSchema.
// It's separated from schema.go to keep the code organized and maintainable.

// Code snippet constants for common validation patterns
const (
	returnNil = "return nil"
)

// Error message templates for generated validation code
// These are used to generate consistent error messages and reduce code duplication
const (
	// Array validation error messages
	errMsgArrayMinItems    = "must have at least %d items, got %%d"
	errMsgArrayMaxItems    = "must have at most %d items, got %%d"
	errMsgArrayMinItemsNil = "must have at least %d items, got 0"

	// Map validation error messages
	errMsgMapMinProps    = "must have at least %d properties, got %%d"
	errMsgMapMaxProps    = "must have at most %d properties, got %%d"
	errMsgMapMinPropsNil = "must have at least %d properties, got 0"
)

var (
	patternCompileMu    sync.Mutex
	patternCompileCache = map[string]bool{}
)

// Go types a string pattern can't apply to: non-string-convertible, or a regex over raw bytes is meaningless.
var nonPatternStringTypes = map[string]bool{
	"time.Time":       true,
	"uuid.UUID":       true,
	"runtime.File":    true,
	"runtime.Date":    true,
	"json.RawMessage": true,
}

// Code generation helpers
func returnNilIfEmptyErrors() string {
	return "if len(errors) == 0 {\n    return nil\n}\nreturn errors"
}

func returnNilIfNoError(validatorVar, alias string) string {
	return fmt.Sprintf("return runtime.ConvertValidatorError(%s.Struct(%s))", validatorVar, alias)
}

func delegateToValidator(castExpr string) string {
	return fmt.Sprintf("if val, ok := any(%s).(runtime.Validator); ok {\n    return val.Validate()\n}\nreturn nil", castExpr)
}

func declareErrorsVar() string {
	return "var errors runtime.ValidationErrors"
}

// Validation generators (in order of appearance in ValidateDecl)

// generateSimpleStructValidation generates validation using validator.Struct()
func generateSimpleStructValidation(s GoSchema, alias, validatorVar string) string {
	return returnNilIfNoError(validatorVar, alias)
}

// generateRefTypeDelegation generates validation that delegates to a RefType
func generateRefTypeDelegation(s GoSchema, alias string) string {
	// Cast to the underlying type to avoid infinite recursion
	// (the current type might implement Validator itself)
	return delegateToValidator(fmt.Sprintf("%s(%s)", s.RefType, alias))
}

// generateTypeAliasDelegation generates validation that delegates to the underlying type
func generateTypeAliasDelegation(s GoSchema, alias string) string {
	// This is a type definition like "type X Y" where Y is another type
	// Cast to the underlying type to avoid infinite recursion
	return delegateToValidator(fmt.Sprintf("%s(%s)", s.TypeDecl(), alias))
}

// generateArrayValidation generates validation for array types
func generateArrayValidation(s GoSchema, alias, validatorVar string) string {
	var lines []string

	// Allow nil if:
	// 1. Explicitly nullable (nullable: true in OpenAPI spec), OR
	// 2. Optional (not required) - computed Constraints.Nullable is true for optional fields
	//
	// Reject nil only if:
	// - Has minItems > 0 AND
	// - Is required (Constraints.Nullable is false or nil)
	isExplicitlyNullable := s.OpenAPISchema != nil && s.OpenAPISchema.Nullable != nil && *s.OpenAPISchema.Nullable
	isOptional := s.Constraints.Nullable != nil && *s.Constraints.Nullable
	hasMinItems := s.Constraints.MinItems != nil && *s.Constraints.MinItems > 0

	if isExplicitlyNullable || isOptional {
		// Nil is allowed for nullable or optional arrays
		lines = append(lines, fmt.Sprintf("if %s == nil {", alias))
		lines = append(lines, "    return nil")
		lines = append(lines, "}")
	} else if hasMinItems {
		// Required array with minItems > 0: nil is invalid
		errMsg := fmt.Sprintf(errMsgArrayMinItemsNil, *s.Constraints.MinItems)
		lines = append(lines, fmt.Sprintf("if %s == nil {", alias))
		lines = append(lines, fmt.Sprintf("    return runtime.NewValidationError(\"Array\", \"%s\")", errMsg))
		lines = append(lines, "}")
	}

	// Collect all constraint violations
	needsErrorCollection := (s.Constraints.MinItems != nil && s.Constraints.MaxItems != nil) ||
		(s.ArrayType != nil && s.ArrayType.NeedsValidation())

	if needsErrorCollection {
		lines = append(lines, declareErrorsVar())
	}

	// Check MinItems constraint
	if s.Constraints.MinItems != nil {
		errMsg := fmt.Sprintf(errMsgArrayMinItems, *s.Constraints.MinItems)
		lines = append(lines, fmt.Sprintf("if len(%s) < %d {", alias, *s.Constraints.MinItems))
		if needsErrorCollection {
			lines = append(lines, fmt.Sprintf("    errors = errors.Add(\"Array\", fmt.Sprintf(\"%s\", len(%s)))", errMsg, alias))
		} else {
			lines = append(lines, fmt.Sprintf("    return runtime.NewValidationError(\"Array\", fmt.Sprintf(\"%s\", len(%s)))", errMsg, alias))
		}
		lines = append(lines, "}")
	}
	// Check MaxItems constraint
	if s.Constraints.MaxItems != nil {
		errMsg := fmt.Sprintf(errMsgArrayMaxItems, *s.Constraints.MaxItems)
		lines = append(lines, fmt.Sprintf("if len(%s) > %d {", alias, *s.Constraints.MaxItems))
		if needsErrorCollection {
			lines = append(lines, fmt.Sprintf("    errors = errors.Add(\"Array\", fmt.Sprintf(\"%s\", len(%s)))", errMsg, alias))
		} else {
			lines = append(lines, fmt.Sprintf("    return runtime.NewValidationError(\"Array\", fmt.Sprintf(\"%s\", len(%s)))", errMsg, alias))
		}
		lines = append(lines, "}")
	}
	// Validate array items if they need validation
	if s.ArrayType != nil && s.ArrayType.NeedsValidation() {
		hasItemTags := len(s.ArrayType.Constraints.ValidationTags) > 0
		hasItemPattern := s.ArrayType.needsPatternValidation()
		lines = append(lines, "for i, item := range "+alias+" {")

		// If items have validation tags, use validator.Var()
		if hasItemTags {
			tags := strings.Join(s.ArrayType.Constraints.ValidationTags, ",")
			lines = append(lines, fmt.Sprintf("    if err := %s.Var(item, \"%s\"); err != nil {", validatorVar, tags))
			lines = append(lines, "        errors = errors.Append(fmt.Sprintf(\"[%d]\", i), err)")
			lines = append(lines, "    }")
		}
		// Enforce a regex pattern on string items.
		if hasItemPattern {
			lines = append(lines, generateElementPatternLines("item", "fmt.Sprintf(\"[%d]\", i)", s.ArrayType)...)
		}
		// Otherwise, try to call Validate() method (for RefTypes, structs, unions)
		if !hasItemTags && !hasItemPattern {
			lines = append(lines, "    if v, ok := any(item).(runtime.Validator); ok {")
			lines = append(lines, "        if err := v.Validate(); err != nil {")
			lines = append(lines, "            errors = errors.Append(fmt.Sprintf(\"[%d]\", i), err)")
			lines = append(lines, "        }")
			lines = append(lines, "    }")
		}

		lines = append(lines, "}")
	}

	// Return collected errors or nil
	if needsErrorCollection {
		lines = append(lines, returnNilIfEmptyErrors())
	} else {
		lines = append(lines, returnNil)
	}
	return strings.Join(lines, "\n")
}

// generateMapValidation generates validation for map types
func generateMapValidation(s GoSchema, alias, validatorVar string) string {
	var lines []string

	// Only allow nil if explicitly nullable OR if there's no minProperties constraint
	// If minProperties > 0 and not explicitly nullable, nil is invalid (nil = 0 properties)
	// Check the OpenAPI schema's nullable field, not the computed Constraints.Nullable
	// (which is true for non-required fields)
	isExplicitlyNullable := s.OpenAPISchema != nil && s.OpenAPISchema.Nullable != nil && *s.OpenAPISchema.Nullable
	hasMinProperties := s.Constraints.MinProperties != nil && *s.Constraints.MinProperties > 0

	if isExplicitlyNullable {
		lines = append(lines, fmt.Sprintf("if %s == nil {", alias))
		lines = append(lines, "    return nil")
		lines = append(lines, "}")
	} else if hasMinProperties {
		// Not explicitly nullable and has minProperties > 0: nil is invalid
		errMsg := fmt.Sprintf(errMsgMapMinPropsNil, *s.Constraints.MinProperties)
		lines = append(lines, fmt.Sprintf("if %s == nil {", alias))
		lines = append(lines, fmt.Sprintf("    return runtime.NewValidationError(\"Map\", \"%s\")", errMsg))
		lines = append(lines, "}")
	}

	// Collect all constraint violations
	needsErrorCollection := (s.Constraints.MinProperties != nil && s.Constraints.MaxProperties != nil) ||
		(s.AdditionalPropertiesType != nil && (len(s.AdditionalPropertiesType.Constraints.ValidationTags) > 0 || s.AdditionalPropertiesType.NeedsValidation()))

	if needsErrorCollection {
		lines = append(lines, declareErrorsVar())
	}

	// Check MinProperties constraint
	if s.Constraints.MinProperties != nil {
		errMsg := fmt.Sprintf(errMsgMapMinProps, *s.Constraints.MinProperties)
		lines = append(lines, fmt.Sprintf("if len(%s) < %d {", alias, *s.Constraints.MinProperties))
		if needsErrorCollection {
			lines = append(lines, fmt.Sprintf("    errors = errors.Add(\"Map\", fmt.Sprintf(\"%s\", len(%s)))", errMsg, alias))
		} else {
			lines = append(lines, fmt.Sprintf("    return runtime.NewValidationError(\"Map\", fmt.Sprintf(\"%s\", len(%s)))", errMsg, alias))
		}
		lines = append(lines, "}")
	}

	// Check MaxProperties constraint
	if s.Constraints.MaxProperties != nil {
		errMsg := fmt.Sprintf(errMsgMapMaxProps, *s.Constraints.MaxProperties)
		lines = append(lines, fmt.Sprintf("if len(%s) > %d {", alias, *s.Constraints.MaxProperties))
		if needsErrorCollection {
			lines = append(lines, fmt.Sprintf("    errors = errors.Add(\"Map\", fmt.Sprintf(\"%s\", len(%s)))", errMsg, alias))
		} else {
			lines = append(lines, fmt.Sprintf("    return runtime.NewValidationError(\"Map\", fmt.Sprintf(\"%s\", len(%s)))", errMsg, alias))
		}
		lines = append(lines, "}")
	}

	// Validate each value if it needs validation
	if s.AdditionalPropertiesType != nil {
		hasValTags := len(s.AdditionalPropertiesType.Constraints.ValidationTags) > 0
		hasValPattern := s.AdditionalPropertiesType.needsPatternValidation()
		if hasValTags || hasValPattern {
			// Primitive values: validator tags and/or a regex pattern.
			lines = append(lines, "for k, v := range "+alias+" {")
			if hasValTags {
				tags := strings.Join(s.AdditionalPropertiesType.Constraints.ValidationTags, ",")
				lines = append(lines, fmt.Sprintf("    if err := %s.Var(v, \"%s\"); err != nil {", validatorVar, tags))
				lines = append(lines, "        errors = errors.Append(k, err)")
				lines = append(lines, "    }")
			}
			if hasValPattern {
				lines = append(lines, generateElementPatternLines("v", "k", s.AdditionalPropertiesType)...)
			}
			lines = append(lines, "}")
			lines = append(lines, returnNilIfEmptyErrors())
		} else if s.AdditionalPropertiesType.NeedsValidation() {
			// For complex types (structs, unions, etc.), call Validate() method
			lines = append(lines, "for k, v := range "+alias+" {")
			lines = append(lines, "    if validator, ok := any(v).(runtime.Validator); ok {")
			lines = append(lines, "        if err := validator.Validate(); err != nil {")
			lines = append(lines, "            errors = errors.Append(k, err)")
			lines = append(lines, "        }")
			lines = append(lines, "    }")
			lines = append(lines, "}")
			lines = append(lines, returnNilIfEmptyErrors())
		} else if needsErrorCollection {
			// We have constraints but no value validation
			lines = append(lines, returnNilIfEmptyErrors())
		} else {
			lines = append(lines, returnNil)
		}
	} else if needsErrorCollection {
		// We have constraints but no additionalProperties validation
		lines = append(lines, returnNilIfEmptyErrors())
	} else {
		lines = append(lines, returnNil)
	}
	return strings.Join(lines, "\n")
}

// generateNonStructValidation generates validation for non-struct types (slices, primitives)
func generateNonStructValidation(s GoSchema, alias, validatorVar string) string {
	typeDecl := s.TypeDecl()
	var lines []string

	// For other non-struct types (slices, primitives)
	if strings.HasPrefix(typeDecl, "[]") || len(s.Properties) == 0 {
		hasTags := len(s.Constraints.ValidationTags) > 0
		hasPattern := s.needsPatternValidation()

		// When both tags and a pattern apply, collect errors so neither check
		// short-circuits the other.
		if hasTags && hasPattern {
			tags := strings.Join(s.Constraints.ValidationTags, ",")
			lines = append(lines, declareErrorsVar())
			lines = append(lines, fmt.Sprintf("if err := %s.Var(%s, \"%s\"); err != nil {", validatorVar, alias, tags))
			lines = append(lines, "    errors = errors.Append(\"\", err)")
			lines = append(lines, "}")
			lines = append(lines, fmt.Sprintf("if err := runtime.ValidatePattern(%s, %s); err != nil {", alias, goStringLiteral(*s.Constraints.Pattern)))
			lines = append(lines, "    errors = errors.Append(\"\", err)")
			lines = append(lines, "}")
			lines = append(lines, returnNilIfEmptyErrors())
			return strings.Join(lines, "\n")
		}

		// Check if the schema itself has validation tags (for primitive types)
		if hasTags {
			tags := strings.Join(s.Constraints.ValidationTags, ",")
			lines = append(lines, fmt.Sprintf("if err := %s.Var(%s, \"%s\"); err != nil {", validatorVar, alias, tags))
			lines = append(lines, "    return err")
			lines = append(lines, "}")
			lines = append(lines, returnNil)
			return strings.Join(lines, "\n")
		}

		// Otherwise enforce just the regex pattern, if present.
		if hasPattern {
			lines = append(lines, fmt.Sprintf("if err := runtime.ValidatePattern(%s, %s); err != nil {", alias, goStringLiteral(*s.Constraints.Pattern)))
			lines = append(lines, "    return err")
			lines = append(lines, "}")
			lines = append(lines, returnNil)
			return strings.Join(lines, "\n")
		}

		return returnNil
	}

	// For struct types, use validator.Struct()
	return returnNilIfNoError(validatorVar, alias)
}

// generateCustomPropertyValidation generates custom validation for struct properties
func generateCustomPropertyValidation(s GoSchema, alias, validatorVar string) string {
	var lines []string

	// Generate custom validation for each property
	// Collect all errors instead of returning early
	lines = append(lines, declareErrorsVar())
	for _, prop := range s.Properties {
		if prop.needsCustomValidation() {
			// Check if this is an array property with items that need validation
			if prop.Schema.ArrayType != nil && prop.Schema.ArrayType.NeedsValidation() {
				lines = append(lines, generateArrayPropertyValidation(alias, prop, validatorVar)...)
			} else if prop.Schema.AdditionalPropertiesType != nil && prop.Schema.AdditionalPropertiesType.NeedsValidation() {
				// Check if this is a map property with values that need validation
				lines = append(lines, generateMapPropertyValidation(alias, prop, validatorVar)...)
			} else {
				// Property needs custom validation - call Validate() method
				if prop.IsPointerType() {
					lines = append(lines, fmt.Sprintf("if %s.%s != nil {", alias, prop.GoName))
					lines = append(lines, fmt.Sprintf("    if v, ok := any(%s.%s).(runtime.Validator); ok {", alias, prop.GoName))
					lines = append(lines, "        if err := v.Validate(); err != nil {")
					lines = append(lines, fmt.Sprintf("            errors = errors.Append(\"%s\", err)", prop.GoName))
					lines = append(lines, "        }")
					lines = append(lines, "    }")
					lines = append(lines, "}")
				} else {
					// For non-pointer types, we still need to handle the case where the field
					// might be nil (e.g., slices, maps, interfaces).
					// If the field is optional (nullable), we should check for nil before validating.
					// This is safe because:
					// - For structs: they can't be nil (unless they're interfaces), so the check is a no-op
					// - For slices/maps: they can be nil, and we want to skip validation if they are
					// - For interfaces: they can be nil, and we want to skip validation if they are
					isOptional := prop.Constraints.Nullable != nil && *prop.Constraints.Nullable

					if isOptional {
						// For optional fields, check if nil before validating
						// Use type assertion to check if the value implements Validator
						// If it does and is not nil, validate it
						lines = append(lines, fmt.Sprintf("if v, ok := any(%s.%s).(runtime.Validator); ok && v != nil {", alias, prop.GoName))
						lines = append(lines, "    if err := v.Validate(); err != nil {")
						lines = append(lines, fmt.Sprintf("        errors = errors.Append(\"%s\", err)", prop.GoName))
						lines = append(lines, "    }")
						lines = append(lines, "}")
					} else {
						lines = append(lines, fmt.Sprintf("if v, ok := any(%s.%s).(runtime.Validator); ok {", alias, prop.GoName))
						lines = append(lines, "    if err := v.Validate(); err != nil {")
						lines = append(lines, fmt.Sprintf("        errors = errors.Append(\"%s\", err)", prop.GoName))
						lines = append(lines, "    }")
						lines = append(lines, "}")
					}
				}
			}
		} else {
			// Primitive property: may carry validator tags and/or a regex pattern.
			lines = append(lines, generatePrimitivePropertyValidation(alias, prop, validatorVar)...)
		}
	}

	lines = append(lines, returnNilIfEmptyErrors())
	return strings.Join(lines, "\n")
}

// generatePrimitivePropertyValidation emits Var() tag checks and/or a ValidatePattern() check
// for a primitive property. The pattern value is passed as-is: runtime.ValidatePattern derefs
// pointers and skips nil/non-string, so no type conversion or nil-guard is needed here.
func generatePrimitivePropertyValidation(alias string, prop Property, validatorVar string) []string {
	hasTags := len(prop.Constraints.ValidationTags) > 0
	hasPattern := prop.needsPatternValidation()
	if !hasTags && !hasPattern {
		return nil
	}

	field := fmt.Sprintf("%s.%s", alias, prop.GoName)
	var lines []string

	if hasTags {
		tags := strings.Join(prop.Constraints.ValidationTags, ",")
		if prop.IsPointerType() {
			lines = append(lines,
				fmt.Sprintf("if %s != nil {", field),
				fmt.Sprintf("    if err := %s.Var(%s, \"%s\"); err != nil {", validatorVar, field, tags),
				fmt.Sprintf("        errors = errors.Append(\"%s\", err)", prop.GoName),
				"    }",
				"}")
		} else {
			lines = append(lines,
				fmt.Sprintf("if err := %s.Var(%s, \"%s\"); err != nil {", validatorVar, field, tags),
				fmt.Sprintf("    errors = errors.Append(\"%s\", err)", prop.GoName),
				"}")
		}
	}

	if hasPattern {
		lines = append(lines,
			fmt.Sprintf("if err := runtime.ValidatePattern(%s, %s); err != nil {", field, goStringLiteral(*prop.Constraints.Pattern)),
			fmt.Sprintf("    errors = errors.Append(\"%s\", err)", prop.GoName),
			"}")
	}

	return lines
}

// generateElementPatternLines emits a ValidatePattern check for one collection element
// (array item or map value). The element is passed as-is; runtime.ValidatePattern derefs
// pointers and skips nil/non-string. keyExpr is the error field-key expression.
func generateElementPatternLines(itemExpr, keyExpr string, elem *GoSchema) []string {
	return []string{
		fmt.Sprintf("    if err := runtime.ValidatePattern(%s, %s); err != nil {", itemExpr, goStringLiteral(*elem.Constraints.Pattern)),
		fmt.Sprintf("        errors = errors.Append(%s, err)", keyExpr),
		"    }",
	}
}

// generateArrayPropertyValidation generates validation code for an array property
func generateArrayPropertyValidation(alias string, prop Property, validatorVar string) []string {
	var lines []string
	fieldAccess := fmt.Sprintf("%s.%s", alias, prop.GoName)

	// Check for nil before iterating
	hasItemTags := len(prop.Schema.ArrayType.Constraints.ValidationTags) > 0
	hasItemPattern := prop.Schema.ArrayType.needsPatternValidation()
	lines = append(lines, fmt.Sprintf("for i, item := range %s {", fieldAccess))

	// If items have validation tags, use validator.Var()
	if hasItemTags {
		tags := strings.Join(prop.Schema.ArrayType.Constraints.ValidationTags, ",")
		lines = append(lines, fmt.Sprintf("    if err := %s.Var(item, \"%s\"); err != nil {", validatorVar, tags))
		lines = append(lines, fmt.Sprintf("        errors = errors.Append(fmt.Sprintf(\"%s[%%d]\", i), err)", prop.GoName))
		lines = append(lines, "    }")
	}

	// Enforce a regex pattern on string items.
	if hasItemPattern {
		keyExpr := fmt.Sprintf("fmt.Sprintf(\"%s[%%d]\", i)", prop.GoName)
		lines = append(lines, generateElementPatternLines("item", keyExpr, prop.Schema.ArrayType)...)
	}

	// Otherwise, try to call Validate() method (for RefTypes, structs, unions)
	if !hasItemTags && !hasItemPattern {
		lines = append(lines, "    if v, ok := any(item).(runtime.Validator); ok {")
		lines = append(lines, "        if err := v.Validate(); err != nil {")
		lines = append(lines, fmt.Sprintf("            errors = errors.Append(fmt.Sprintf(\"%s[%%d]\", i), err)", prop.GoName))
		lines = append(lines, "        }")
		lines = append(lines, "    }")
	}

	lines = append(lines, "}")
	return lines
}

// generateMapPropertyValidation generates validation code for a map property
func generateMapPropertyValidation(alias string, prop Property, validatorVar string) []string {
	var lines []string
	fieldAccess := fmt.Sprintf("%s.%s", alias, prop.GoName)

	// Iterate over map values
	hasValTags := len(prop.Schema.AdditionalPropertiesType.Constraints.ValidationTags) > 0
	hasValPattern := prop.Schema.AdditionalPropertiesType.needsPatternValidation()
	lines = append(lines, fmt.Sprintf("for k, v := range %s {", fieldAccess))

	// If values have validation tags, use validator.Var()
	if hasValTags {
		tags := strings.Join(prop.Schema.AdditionalPropertiesType.Constraints.ValidationTags, ",")
		lines = append(lines, fmt.Sprintf("    if err := %s.Var(v, \"%s\"); err != nil {", validatorVar, tags))
		lines = append(lines, fmt.Sprintf("        errors = errors.Append(fmt.Sprintf(\"%s[%%s]\", k), err)", prop.GoName))
		lines = append(lines, "    }")
	}

	// Enforce a regex pattern on string values.
	if hasValPattern {
		keyExpr := fmt.Sprintf("fmt.Sprintf(\"%s[%%s]\", k)", prop.GoName)
		lines = append(lines, generateElementPatternLines("v", keyExpr, prop.Schema.AdditionalPropertiesType)...)
	}

	// Otherwise, try to call Validate() method (for RefTypes, structs, unions)
	if !hasValTags && !hasValPattern {
		lines = append(lines, "    if validator, ok := any(v).(runtime.Validator); ok {")
		lines = append(lines, "        if err := validator.Validate(); err != nil {")
		lines = append(lines, fmt.Sprintf("            errors = errors.Append(fmt.Sprintf(\"%s[%%s]\", k), err)", prop.GoName))
		lines = append(lines, "        }")
		lines = append(lines, "    }")
	}

	lines = append(lines, "}")
	return lines
}

// isStructType checks if this schema represents a struct type
func isStructType(s GoSchema) bool {
	typeDecl := s.TypeDecl()
	return strings.HasPrefix(typeDecl, "struct") && len(s.Properties) > 0
}

// canUseSimpleStructValidation checks if we can use the optimized validator.Struct() approach
func canUseSimpleStructValidation(s GoSchema) bool {
	typeDecl := s.TypeDecl()
	if !strings.HasPrefix(typeDecl, "struct") || len(s.Properties) == 0 || s.ContainsUnions() {
		return false
	}

	// A property needing custom validation or a regex pattern rules out validator.Struct().
	for _, prop := range s.Properties {
		if prop.needsCustomValidation() || prop.needsPatternValidation() {
			return false
		}
	}
	return true
}

// isArrayType checks if this schema represents an array type
func isArrayType(s GoSchema) bool {
	return s.ArrayType != nil && strings.HasPrefix(s.TypeDecl(), "[]")
}

// isRefTypeDelegation checks if this schema should delegate to a RefType
func isRefTypeDelegation(s GoSchema) bool {
	return s.RefType != "" && !s.IsExternalRef()
}

// isTypeAliasDelegation checks if this schema is a type alias that should delegate
func isTypeAliasDelegation(s GoSchema) bool {
	typeDecl := s.TypeDecl()
	return len(s.Properties) > 0 &&
		!strings.HasPrefix(typeDecl, "struct") &&
		!strings.HasPrefix(typeDecl, "map[") &&
		!strings.HasPrefix(typeDecl, "[]")
}

// isMapType checks if this schema represents a map type
func isMapType(s GoSchema) bool {
	typeDecl := s.TypeDecl()
	return strings.HasPrefix(typeDecl, "map[")
}

// hasCustomValidation reports whether any property needs custom validation, including a regex pattern.
func hasCustomValidation(s GoSchema) bool {
	for _, prop := range s.Properties {
		if prop.needsCustomValidation() || prop.needsPatternValidation() {
			return true
		}
	}
	return false
}

// isPatternValidatable reports whether emitting a regex check for goType is worthwhile.
// It filters obvious non-strings (slices, maps, non-string primitives) at generation time;
// runtime.ValidatePattern is the final authority and safely skips anything non-string-kinded.
func isPatternValidatable(goType string) bool {
	base := strings.TrimPrefix(goType, "*")
	if strings.HasPrefix(base, "[]") || strings.HasPrefix(base, "map[") {
		return false
	}

	if nonPatternStringTypes[base] {
		return false
	}

	// Non-string primitives (int, bool, time.Time, ...) can't carry a regex.
	if isPrimitiveType(base) && base != "string" {
		return false
	}
	return true
}

// goStringLiteral renders s as a Go literal, preferring a raw string unless s contains a backtick.
func goStringLiteral(s string) string {
	if !strings.ContainsRune(s, '`') {
		return "`" + s + "`"
	}
	return fmt.Sprintf("%q", s)
}

// patternCompiles reports (and caches) whether Go's RE2 engine can compile pattern; unsupported ones are skipped with a warning.
func patternCompiles(pattern string) bool {
	patternCompileMu.Lock()
	defer patternCompileMu.Unlock()
	if ok, seen := patternCompileCache[pattern]; seen {
		return ok
	}

	_, err := regexp.Compile(pattern)
	ok := err == nil
	patternCompileCache[pattern] = ok
	if !ok {
		slog.Warn("skipping unsupported regex pattern in generated Validate(): Go's regexp engine (RE2) could not compile it",
			"pattern", pattern, "error", err)
	}
	return ok
}
