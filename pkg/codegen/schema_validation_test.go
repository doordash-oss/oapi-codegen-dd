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
	"strings"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
)

// normalizeCode removes extra whitespace and normalizes indentation for comparison
func normalizeCode(code string) string {
	lines := strings.Split(code, "\n")
	var normalized []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return strings.Join(normalized, "\n")
}

// visualDiff creates a side-by-side comparison of expected vs actual
func visualDiff(expected, actual string) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(strings.Repeat("=", 80))
	b.WriteString("\n")
	b.WriteString("EXPECTED:\n")
	b.WriteString(strings.Repeat("-", 80))
	b.WriteString("\n")
	b.WriteString(expected)
	b.WriteString("\n")
	b.WriteString(strings.Repeat("-", 80))
	b.WriteString("\n")
	b.WriteString("ACTUAL:\n")
	b.WriteString(strings.Repeat("-", 80))
	b.WriteString("\n")
	b.WriteString(actual)
	b.WriteString("\n")
	b.WriteString(strings.Repeat("=", 80))
	b.WriteString("\n")
	return b.String()
}

// assertCodeEqual compares two code snippets after normalization
func assertCodeEqual(t *testing.T, expected, actual string, msgAndArgs ...any) {
	t.Helper()

	normalizedExpected := normalizeCode(expected)
	normalizedActual := normalizeCode(actual)

	if normalizedExpected != normalizedActual {
		t.Errorf("Code mismatch:%s", visualDiff(normalizedExpected, normalizedActual))
		if len(msgAndArgs) > 0 {
			t.Logf("Additional context: %v", msgAndArgs)
		}
		t.FailNow()
	}
}

func TestGoSchema_ValidateDecl_ArrayWithMinItems(t *testing.T) {
	minItems := int64(1)
	schema := GoSchema{
		ArrayType: &GoSchema{
			GoType: "string",
		},
		Constraints: Constraints{
			MinItems: &minItems,
		},
	}

	result := schema.ValidateDecl("p", "validate")
	expected := `
		if p == nil {
			return runtime.NewValidationError("Array", "must have at least 1 items, got 0")
		}
		if len(p) < 1 {
			return runtime.NewValidationError("Array", fmt.Sprintf("must have at least 1 items, got %d", len(p)))
		}
		return nil
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_ArrayWithMaxItems(t *testing.T) {
	maxItems := int64(100)
	schema := GoSchema{
		ArrayType: &GoSchema{
			GoType: "string",
		},
		Constraints: Constraints{
			MaxItems: &maxItems,
		},
	}

	result := schema.ValidateDecl("p", "validate")
	expected := `
		if len(p) > 100 {
			return runtime.NewValidationError("Array", fmt.Sprintf("must have at most 100 items, got %d", len(p)))
		}
		return nil
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_ArrayWithMinAndMaxItems(t *testing.T) {
	minItems := int64(1)
	maxItems := int64(100)
	schema := GoSchema{
		ArrayType: &GoSchema{
			GoType: "string",
		},
		Constraints: Constraints{
			MinItems: &minItems,
			MaxItems: &maxItems,
		},
	}

	result := schema.ValidateDecl("p", "validate")
	expected := `
		if p == nil {
			return runtime.NewValidationError("Array", "must have at least 1 items, got 0")
		}
		var errors runtime.ValidationErrors
		if len(p) < 1 {
			errors = errors.Add("Array", fmt.Sprintf("must have at least 1 items, got %d", len(p)))
		}
		if len(p) > 100 {
			errors = errors.Add("Array", fmt.Sprintf("must have at most 100 items, got %d", len(p)))
		}
		if len(errors) == 0 {
			return nil
		}
		return errors
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_NullableArrayWithConstraints(t *testing.T) {
	minItems := int64(1)
	nullable := true
	schema := GoSchema{
		ArrayType: &GoSchema{
			GoType: "string",
		},
		Constraints: Constraints{
			MinItems: &minItems,
			Nullable: ptr(true),
		},
		OpenAPISchema: &base.Schema{
			Nullable: &nullable,
		},
	}

	result := schema.ValidateDecl("p", "validate")
	expected := `
		if p == nil {
			return nil
		}
		if len(p) < 1 {
			return runtime.NewValidationError("Array", fmt.Sprintf("must have at least 1 items, got %d", len(p)))
		}
		return nil
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_ArrayWithRefTypeItems(t *testing.T) {
	schema := GoSchema{
		ArrayType: &GoSchema{
			RefType: "Payment",
		},
	}

	result := schema.ValidateDecl("p", "validate")
	expected := `
		var errors runtime.ValidationErrors
		for i, item := range p {
			if v, ok := any(item).(runtime.Validator); ok {
				if err := v.Validate(); err != nil {
					errors = errors.Append(fmt.Sprintf("[%d]", i), err)
				}
			}
		}
		if len(errors) == 0 {
			return nil
		}
		return errors
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_ArrayWithExternalRefTypeItems(t *testing.T) {
	schema := GoSchema{
		ArrayType: &GoSchema{
			RefType: "external.Payment", // External ref (contains ".")
		},
	}

	result := schema.ValidateDecl("p", "validate")
	expected := `return nil`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_ArrayWithValidationTagsOnItems(t *testing.T) {
	schema := GoSchema{
		ArrayType: &GoSchema{
			GoType: "string",
			Constraints: Constraints{
				ValidationTags: []string{"omitempty", "min=3"},
			},
		},
	}

	result := schema.ValidateDecl("p", "validate")
	expected := `
		var errors runtime.ValidationErrors
		for i, item := range p {
			if err := validate.Var(item, "omitempty,min=3"); err != nil {
				errors = errors.Append(fmt.Sprintf("[%d]", i), err)
			}
		}
		if len(errors) == 0 {
			return nil
		}
		return errors
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_ArrayWithConstraintsAndValidationTags(t *testing.T) {
	minItems := int64(1)
	maxItems := int64(100)
	schema := GoSchema{
		ArrayType: &GoSchema{
			GoType: "string",
			Constraints: Constraints{
				ValidationTags: []string{"min=3"},
			},
		},
		Constraints: Constraints{
			MinItems: &minItems,
			MaxItems: &maxItems,
		},
	}

	result := schema.ValidateDecl("p", "validate")
	expected := `
		if p == nil {
			return runtime.NewValidationError("Array", "must have at least 1 items, got 0")
		}
		var errors runtime.ValidationErrors
		if len(p) < 1 {
			errors = errors.Add("Array", fmt.Sprintf("must have at least 1 items, got %d", len(p)))
		}
		if len(p) > 100 {
			errors = errors.Add("Array", fmt.Sprintf("must have at most 100 items, got %d", len(p)))
		}
		for i, item := range p {
			if err := validate.Var(item, "min=3"); err != nil {
				errors = errors.Append(fmt.Sprintf("[%d]", i), err)
			}
		}
		if len(errors) == 0 {
			return nil
		}
		return errors
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_MapWithMinProperties(t *testing.T) {
	minProps := int64(2)
	schema := GoSchema{
		GoType: "map[string]any",
		Constraints: Constraints{
			MinProperties: &minProps,
		},
	}

	result := schema.ValidateDecl("m", "validate")
	expected := `
		if m == nil {
			return runtime.NewValidationError("Map", "must have at least 2 properties, got 0")
		}
		if len(m) < 2 {
			return runtime.NewValidationError("Map", fmt.Sprintf("must have at least 2 properties, got %d", len(m)))
		}
		return nil
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_MapWithMaxProperties(t *testing.T) {
	maxProps := int64(10)
	schema := GoSchema{
		GoType: "map[string]any",
		Constraints: Constraints{
			MaxProperties: &maxProps,
		},
	}

	result := schema.ValidateDecl("m", "validate")
	expected := `
		if len(m) > 10 {
			return runtime.NewValidationError("Map", fmt.Sprintf("must have at most 10 properties, got %d", len(m)))
		}
		return nil
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_MapWithMinAndMaxProperties(t *testing.T) {
	minProps := int64(2)
	maxProps := int64(10)
	schema := GoSchema{
		GoType: "map[string]any",
		Constraints: Constraints{
			MinProperties: &minProps,
			MaxProperties: &maxProps,
		},
	}

	result := schema.ValidateDecl("m", "validate")
	expected := `
		if m == nil {
			return runtime.NewValidationError("Map", "must have at least 2 properties, got 0")
		}
		var errors runtime.ValidationErrors
		if len(m) < 2 {
			errors = errors.Add("Map", fmt.Sprintf("must have at least 2 properties, got %d", len(m)))
		}
		if len(m) > 10 {
			errors = errors.Add("Map", fmt.Sprintf("must have at most 10 properties, got %d", len(m)))
		}
		if len(errors) == 0 {
			return nil
		}
		return errors
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_NullableMapWithConstraints(t *testing.T) {
	minProps := int64(2)
	nullable := true
	schema := GoSchema{
		GoType: "map[string]any",
		Constraints: Constraints{
			MinProperties: &minProps,
			Nullable:      ptr(true),
		},
		OpenAPISchema: &base.Schema{
			Nullable: &nullable,
		},
	}

	result := schema.ValidateDecl("m", "validate")
	expected := `
		if m == nil {
			return nil
		}
		if len(m) < 2 {
			return runtime.NewValidationError("Map", fmt.Sprintf("must have at least 2 properties, got %d", len(m)))
		}
		return nil
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_MapWithRefTypeValues(t *testing.T) {
	schema := GoSchema{
		GoType: "map[string]Payment",
		AdditionalPropertiesType: &GoSchema{
			RefType: "Payment",
		},
	}

	result := schema.ValidateDecl("m", "validate")
	expected := `
		var errors runtime.ValidationErrors
		for k, v := range m {
			if validator, ok := any(v).(runtime.Validator); ok {
				if err := validator.Validate(); err != nil {
					errors = errors.Append(k, err)
				}
			}
		}
		if len(errors) == 0 {
			return nil
		}
		return errors
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_MapWithExternalRefTypeValues(t *testing.T) {
	schema := GoSchema{
		GoType: "map[string]external.Payment",
		AdditionalPropertiesType: &GoSchema{
			RefType: "external.Payment", // External ref
		},
	}

	result := schema.ValidateDecl("m", "validate")
	expected := `return nil`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_EmptySliceType(t *testing.T) {
	schema := GoSchema{
		GoType: "[]string",
	}

	result := schema.ValidateDecl("s", "validate")
	expected := `return nil`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_EmptyStruct(t *testing.T) {
	schema := GoSchema{
		GoType:     "struct{}",
		Properties: []Property{},
	}

	result := schema.ValidateDecl("s", "validate")
	expected := `return nil`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_StructWithValidatorStruct(t *testing.T) {
	schema := GoSchema{
		GoType: "struct { Name string `json:\"name\" validate:\"required\"` }",
		Properties: []Property{
			{
				GoName:        "Name",
				JsonFieldName: "name",
				Schema: GoSchema{
					GoType: "string",
				},
				Constraints: Constraints{
					ValidationTags: []string{"required"},
				},
			},
		},
	}

	result := schema.ValidateDecl("s", "validate")
	expected := `return runtime.ConvertValidatorError(validate.Struct(s))`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_StructWithRefTypeProperty(t *testing.T) {
	schema := GoSchema{
		GoType: "struct { User User }",
		Properties: []Property{
			{
				GoName:        "User",
				JsonFieldName: "user",
				Schema: GoSchema{
					RefType: "User",
				},
			},
		},
	}

	result := schema.ValidateDecl("s", "validate")
	expected := `
		var errors runtime.ValidationErrors
		if v, ok := any(s.User).(runtime.Validator); ok {
			if err := v.Validate(); err != nil {
				errors = errors.Append("User", err)
			}
		}
		if len(errors) == 0 {
			return nil
		}
		return errors
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_StructWithPointerRefTypeProperty(t *testing.T) {
	schema := GoSchema{
		GoType: "struct { User *User }",
		Properties: []Property{
			{
				GoName:        "User",
				JsonFieldName: "user",
				Schema: GoSchema{
					RefType: "User",
				},
				Constraints: Constraints{
					Nullable: ptr(true),
				},
			},
		},
	}

	result := schema.ValidateDecl("s", "validate")
	expected := `
		var errors runtime.ValidationErrors
		if s.User != nil {
			if v, ok := any(s.User).(runtime.Validator); ok {
				if err := v.Validate(); err != nil {
					errors = errors.Append("User", err)
				}
			}
		}
		if len(errors) == 0 {
			return nil
		}
		return errors
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_StructWithValidationTagsProperty(t *testing.T) {
	schema := GoSchema{
		GoType: "struct { Name string }",
		Properties: []Property{
			{
				GoName:        "Name",
				JsonFieldName: "name",
				Schema: GoSchema{
					GoType: "string",
				},
				Constraints: Constraints{
					ValidationTags: []string{"required", "min=3"},
				},
			},
		},
	}

	result := schema.ValidateDecl("s", "validate")
	// Primitive types with validation tags use validate.Struct()
	expected := `return runtime.ConvertValidatorError(validate.Struct(s))`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_StructWithPointerValidationTagsProperty(t *testing.T) {
	schema := GoSchema{
		GoType: "struct { Name *string }",
		Properties: []Property{
			{
				GoName:        "Name",
				JsonFieldName: "name",
				Schema: GoSchema{
					GoType: "string",
				},
				Constraints: Constraints{
					ValidationTags: []string{"required", "min=3"},
					Nullable:       ptr(true),
				},
			},
		},
	}

	result := schema.ValidateDecl("s", "validate")
	// Primitive types with validation tags use validate.Struct()
	expected := `return runtime.ConvertValidatorError(validate.Struct(s))`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_StructWithCustomTypeValidationTagsProperty(t *testing.T) {
	// Custom type (not a primitive) with validation tags should use custom validation
	schema := GoSchema{
		GoType: "struct { Payment PaymentType }",
		Properties: []Property{
			{
				GoName:        "Payment",
				JsonFieldName: "payment",
				Schema: GoSchema{
					GoType: "PaymentType", // Custom type
				},
				Constraints: Constraints{
					ValidationTags: []string{"required"},
				},
			},
		},
	}

	result := schema.ValidateDecl("s", "validate")
	// Custom types call Validate() method, not Var() with tags
	expected := `
		var errors runtime.ValidationErrors
		if v, ok := any(s.Payment).(runtime.Validator); ok {
			if err := v.Validate(); err != nil {
				errors = errors.Append("Payment", err)
			}
		}
		if len(errors) == 0 {
			return nil
		}
		return errors
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_StructWithMixedProperties(t *testing.T) {
	// Mix of RefType and validation tags
	schema := GoSchema{
		GoType: "struct { User User; Name string }",
		Properties: []Property{
			{
				GoName:        "User",
				JsonFieldName: "user",
				Schema: GoSchema{
					RefType: "User",
				},
			},
			{
				GoName:        "Name",
				JsonFieldName: "name",
				Schema: GoSchema{
					GoType: "string",
				},
				Constraints: Constraints{
					ValidationTags: []string{"required"},
				},
			},
		},
	}

	result := schema.ValidateDecl("s", "validate")
	// User needs custom validation (RefType), Name has validation tags
	// Both get validated and errors collected
	expected := `
		var errors runtime.ValidationErrors
		if v, ok := any(s.User).(runtime.Validator); ok {
			if err := v.Validate(); err != nil {
				errors = errors.Append("User", err)
			}
		}
		if err := validate.Var(s.Name, "required"); err != nil {
			errors = errors.Append("Name", err)
		}
		if len(errors) == 0 {
			return nil
		}
		return errors
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_NoValidation(t *testing.T) {
	// Struct with no validation needs
	schema := GoSchema{
		GoType: "struct { Name string }",
		Properties: []Property{
			{
				GoName:        "Name",
				JsonFieldName: "name",
				Schema: GoSchema{
					GoType: "string",
				},
			},
		},
	}

	result := schema.ValidateDecl("s", "validate")
	// No validation tags, no RefType, so use validate.Struct()
	expected := `return runtime.ConvertValidatorError(validate.Struct(s))`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_ArrayWithNullableAndRefTypeItems(t *testing.T) {
	minItems := int64(1)
	nullable := true
	schema := GoSchema{
		ArrayType: &GoSchema{
			RefType: "Payment",
		},
		Constraints: Constraints{
			MinItems: &minItems,
			Nullable: ptr(true),
		},
		OpenAPISchema: &base.Schema{
			Nullable: &nullable,
		},
	}

	result := schema.ValidateDecl("p", "validate")
	expected := `
		if p == nil {
			return nil
		}
		var errors runtime.ValidationErrors
		if len(p) < 1 {
			errors = errors.Add("Array", fmt.Sprintf("must have at least 1 items, got %d", len(p)))
		}
		for i, item := range p {
			if v, ok := any(item).(runtime.Validator); ok {
				if err := v.Validate(); err != nil {
					errors = errors.Append(fmt.Sprintf("[%d]", i), err)
				}
			}
		}
		if len(errors) == 0 {
			return nil
		}
		return errors
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_MapWithConstraintsAndRefTypeValues(t *testing.T) {
	minProps := int64(1)
	maxProps := int64(10)
	schema := GoSchema{
		GoType: "map[string]Payment",
		AdditionalPropertiesType: &GoSchema{
			RefType: "Payment",
		},
		Constraints: Constraints{
			MinProperties: &minProps,
			MaxProperties: &maxProps,
		},
	}

	result := schema.ValidateDecl("m", "validate")
	expected := `
		if m == nil {
			return runtime.NewValidationError("Map", "must have at least 1 properties, got 0")
		}
		var errors runtime.ValidationErrors
		if len(m) < 1 {
			errors = errors.Add("Map", fmt.Sprintf("must have at least 1 properties, got %d", len(m)))
		}
		if len(m) > 10 {
			errors = errors.Add("Map", fmt.Sprintf("must have at most 10 properties, got %d", len(m)))
		}
		for k, v := range m {
			if validator, ok := any(v).(runtime.Validator); ok {
				if err := validator.Validate(); err != nil {
					errors = errors.Append(k, err)
				}
			}
		}
		if len(errors) == 0 {
			return nil
		}
		return errors
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_TypeAliasWithRefType(t *testing.T) {
	// Test that type aliases delegate to the underlying type correctly
	// This prevents infinite recursion when the alias type implements Validator
	schema := GoSchema{
		RefType: "UnderlyingType",
		Properties: []Property{
			{GoName: "Field1", JsonFieldName: "field1"},
		},
	}

	result := schema.ValidateDecl("p", "validate")
	expected := `
		if val, ok := any(UnderlyingType(p)).(runtime.Validator); ok {
			return val.Validate()
		}
		return nil
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_TypeAliasWithAliasNameV(t *testing.T) {
	// Test that we don't have variable name collision when alias is "v"
	schema := GoSchema{
		RefType: "UnderlyingType",
		Properties: []Property{
			{GoName: "Field1", JsonFieldName: "field1"},
		},
	}

	result := schema.ValidateDecl("v", "validate")
	expected := `
		if val, ok := any(UnderlyingType(v)).(runtime.Validator); ok {
			return val.Validate()
		}
		return nil
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_TypeBasedOnOtherType(t *testing.T) {
	// Test delegation for "type X Y" where Y is another type (not struct/map/slice)
	schema := GoSchema{
		GoType: "OtherType",
		Properties: []Property{
			{GoName: "Field1", JsonFieldName: "field1"},
		},
	}

	result := schema.ValidateDecl("p", "validate")
	expected := `
		if val, ok := any(OtherType(p)).(runtime.Validator); ok {
			return val.Validate()
		}
		return nil
	`
	assertCodeEqual(t, expected, result)
}

// Tests for lines 151-157: RefType delegation
func TestGoSchema_ValidateDecl_RefTypeDelegation(t *testing.T) {
	// Test that a schema with RefType delegates to the underlying type
	schema := GoSchema{
		RefType: "Payment",
	}

	result := schema.ValidateDecl("p", "validate")
	expected := `
		if val, ok := any(Payment(p)).(runtime.Validator); ok {
			return val.Validate()
		}
		return nil
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_RefTypeDelegation_WithProperties(t *testing.T) {
	// Test that RefType takes precedence even when properties are present
	schema := GoSchema{
		RefType: "User",
		Properties: []Property{
			{GoName: "Name", JsonFieldName: "name"},
			{GoName: "Email", JsonFieldName: "email"},
		},
	}

	result := schema.ValidateDecl("u", "validate")
	expected := `
		if val, ok := any(User(u)).(runtime.Validator); ok {
			return val.Validate()
		}
		return nil
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_ExternalRefType_NoValidation(t *testing.T) {
	// Test that external RefType (contains ".") returns nil without validation
	schema := GoSchema{
		RefType: "external.Payment",
	}

	result := schema.ValidateDecl("p", "validate")
	expected := `return nil`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_ExternalRefType_WithProperties(t *testing.T) {
	// Test that external RefType with properties delegates via the second block (lines 159-167)
	// because the first block (lines 151-157) skips external refs
	schema := GoSchema{
		RefType: "github.com/external/pkg.User",
		Properties: []Property{
			{GoName: "Name", JsonFieldName: "name"},
		},
	}

	result := schema.ValidateDecl("u", "validate")
	// External refs with properties fall through to the second delegation block
	expected := `
		if val, ok := any(github.com/external/pkg.User(u)).(runtime.Validator); ok {
			return val.Validate()
		}
		return nil
	`
	assertCodeEqual(t, expected, result)
}

// Type alias delegation for non-struct/map/slice types
func TestGoSchema_ValidateDecl_TypeAliasDelegation_CustomType(t *testing.T) {
	// Test delegation for "type X Y" where Y is a custom type (not struct/map/slice)
	schema := GoSchema{
		GoType: "CustomType",
		Properties: []Property{
			{GoName: "Field1", JsonFieldName: "field1"},
		},
	}

	result := schema.ValidateDecl("c", "validate")
	expected := `
		if val, ok := any(CustomType(c)).(runtime.Validator); ok {
			return val.Validate()
		}
		return nil
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_TypeAliasDelegation_PrimitiveType(t *testing.T) {
	// Test delegation for "type X string" where X is based on a primitive
	schema := GoSchema{
		GoType: "string",
		Properties: []Property{
			{GoName: "Value", JsonFieldName: "value"},
		},
	}

	result := schema.ValidateDecl("s", "validate")
	expected := `
		if val, ok := any(string(s)).(runtime.Validator); ok {
			return val.Validate()
		}
		return nil
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_NoTypeAliasDelegation_Struct(t *testing.T) {
	// Test that struct types do NOT delegate (they use validate.Struct)
	schema := GoSchema{
		GoType: "struct { Name string }",
		Properties: []Property{
			{GoName: "Name", JsonFieldName: "name"},
		},
	}

	result := schema.ValidateDecl("s", "validate")
	// Should use validate.Struct, not delegation
	expected := `return runtime.ConvertValidatorError(validate.Struct(s))`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_NoTypeAliasDelegation_Map(t *testing.T) {
	// Test that map types do NOT delegate
	schema := GoSchema{
		GoType: "map[string]string",
		Properties: []Property{
			{GoName: "Data", JsonFieldName: "data"},
		},
	}

	result := schema.ValidateDecl("m", "validate")
	// Should return nil (no validation for map without constraints)
	expected := `return nil`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_NoTypeAliasDelegation_Slice(t *testing.T) {
	// Test that slice types do NOT delegate
	schema := GoSchema{
		GoType: "[]string",
		Properties: []Property{
			{GoName: "Items", JsonFieldName: "items"},
		},
	}

	result := schema.ValidateDecl("s", "validate")
	// Should return nil (no validation for slice without constraints)
	expected := `return nil`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_TypeAliasDelegation_MultipleProperties(t *testing.T) {
	// Test delegation with multiple properties
	schema := GoSchema{
		GoType: "BaseType",
		Properties: []Property{
			{GoName: "Field1", JsonFieldName: "field1"},
			{GoName: "Field2", JsonFieldName: "field2"},
			{GoName: "Field3", JsonFieldName: "field3"},
		},
	}

	result := schema.ValidateDecl("b", "validate")
	expected := `
		if val, ok := any(BaseType(b)).(runtime.Validator); ok {
			return val.Validate()
		}
		return nil
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_RefTypePrecedence_OverGoType(t *testing.T) {
	// Test that RefType takes precedence over GoType when both are set
	schema := GoSchema{
		RefType: "RefTypeName",
		GoType:  "string",
		Properties: []Property{
			{GoName: "Value", JsonFieldName: "value"},
		},
	}

	result := schema.ValidateDecl("r", "validate")
	// Should use RefType, not GoType
	expected := `
		if val, ok := any(RefTypeName(r)).(runtime.Validator); ok {
			return val.Validate()
		}
		return nil
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_PrimitiveWithValidationTags(t *testing.T) {
	// Test that primitive types with validation tags generate proper validation code
	schema := GoSchema{
		GoType: "string",
		Constraints: Constraints{
			ValidationTags: []string{"omitempty", "min=4", "max=7"},
		},
	}

	result := schema.ValidateDecl("s", "schemaTypesValidate")
	expected := `
		if err := schemaTypesValidate.Var(s, "omitempty,min=4,max=7"); err != nil {
			return err
		}
		return nil
	`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_PrimitiveWithoutValidationTags(t *testing.T) {
	// Test that primitive types without validation tags just return nil
	schema := GoSchema{
		GoType: "string",
	}

	result := schema.ValidateDecl("s", "schemaTypesValidate")
	expected := `return nil`
	assertCodeEqual(t, expected, result)
}

func TestGoSchema_ValidateDecl_IntegerWithValidationTags(t *testing.T) {
	// Test that integer types with validation tags generate proper validation code
	schema := GoSchema{
		GoType: "int",
		Constraints: Constraints{
			ValidationTags: []string{"omitempty", "gte=1", "lte=100"},
		},
	}

	result := schema.ValidateDecl("i", "schemaTypesValidate")
	expected := `
		if err := schemaTypesValidate.Var(i, "omitempty,gte=1,lte=100"); err != nil {
			return err
		}
		return nil
	`
	assertCodeEqual(t, expected, result)
}

// TestGoSchema_NeedsValidation_StructWithArrayOfCustomTypes tests that a struct
// with only array properties whose item types need validation correctly returns
// true for NeedsValidation(). This is a regression test for the bug where
// DisputesListResponse with Items []DisputeInfo didn't get a Validate() method.
func TestGoSchema_NeedsValidation_StructWithArrayOfCustomTypes(t *testing.T) {
	// Simulate DisputeInfo - a custom type that needs validation
	disputeInfoSchema := GoSchema{
		GoType:  "struct { ID string }",
		RefType: "DisputeInfo",
	}

	// Simulate DisputesListResponse with Items []DisputeInfo
	schema := GoSchema{
		GoType: "struct { Items []DisputeInfo }",
		Properties: []Property{
			{
				GoName: "Items",
				Schema: GoSchema{
					GoType:    "[]DisputeInfo",
					ArrayType: &disputeInfoSchema,
				},
			},
		},
	}

	// The struct should need validation because Items contains DisputeInfo which needs validation
	if !schema.NeedsValidation() {
		t.Error("Expected NeedsValidation() to return true for struct with array of custom types")
	}
}

// TestGoSchema_ValidateDecl_StructWithArrayOfCustomTypes tests that validation
// code is correctly generated for a struct with array properties whose item types
// need validation.
func TestGoSchema_ValidateDecl_StructWithArrayOfCustomTypes(t *testing.T) {
	// Simulate DisputeInfo - a custom type that needs validation
	disputeInfoSchema := GoSchema{
		GoType:  "struct { ID string }",
		RefType: "DisputeInfo",
	}

	// Simulate DisputesListResponse with Items []DisputeInfo
	schema := GoSchema{
		GoType: "struct { Items []DisputeInfo }",
		Properties: []Property{
			{
				GoName: "Items",
				Schema: GoSchema{
					GoType:    "[]DisputeInfo",
					ArrayType: &disputeInfoSchema,
				},
			},
		},
	}

	result := schema.ValidateDecl("d", "typesValidator")
	expected := `
		var errors runtime.ValidationErrors
		for i, item := range d.Items {
			if v, ok := any(item).(runtime.Validator); ok {
				if err := v.Validate(); err != nil {
					errors = errors.Append(fmt.Sprintf("Items[%d]", i), err)
				}
			}
		}
		if len(errors) == 0 {
			return nil
		}
		return errors
	`
	assertCodeEqual(t, expected, result)
}

// TestGoSchema_ValidateDecl_StructWithMultipleArraysOfCustomTypes tests validation
// for a struct with multiple array properties.
func TestGoSchema_ValidateDecl_StructWithMultipleArraysOfCustomTypes(t *testing.T) {
	// Simulate two custom types
	disputeInfoSchema := GoSchema{
		GoType:  "struct { ID string }",
		RefType: "DisputeInfo",
	}
	linkDescriptionSchema := GoSchema{
		GoType:  "struct { Href string }",
		RefType: "LinkDescription",
	}

	// Simulate DisputesListResponse with Items []DisputeInfo and Links []LinkDescription
	schema := GoSchema{
		GoType: "struct { Items []DisputeInfo; Links []LinkDescription }",
		Properties: []Property{
			{
				GoName: "Items",
				Schema: GoSchema{
					GoType:    "[]DisputeInfo",
					ArrayType: &disputeInfoSchema,
				},
			},
			{
				GoName: "Links",
				Schema: GoSchema{
					GoType:    "[]LinkDescription",
					ArrayType: &linkDescriptionSchema,
				},
			},
		},
	}

	result := schema.ValidateDecl("d", "typesValidator")
	expected := `
		var errors runtime.ValidationErrors
		for i, item := range d.Items {
			if v, ok := any(item).(runtime.Validator); ok {
				if err := v.Validate(); err != nil {
					errors = errors.Append(fmt.Sprintf("Items[%d]", i), err)
				}
			}
		}
		for i, item := range d.Links {
			if v, ok := any(item).(runtime.Validator); ok {
				if err := v.Validate(); err != nil {
					errors = errors.Append(fmt.Sprintf("Links[%d]", i), err)
				}
			}
		}
		if len(errors) == 0 {
			return nil
		}
		return errors
	`
	assertCodeEqual(t, expected, result)
}

// TestGoSchema_ValidateDecl_StructWithMapOfCustomTypes tests validation
// for a struct with map properties whose value types need validation.
func TestGoSchema_ValidateDecl_StructWithMapOfCustomTypes(t *testing.T) {
	// Simulate a custom type
	disputeInfoSchema := GoSchema{
		GoType:  "struct { ID string }",
		RefType: "DisputeInfo",
	}

	// Simulate a struct with Data map[string]DisputeInfo
	schema := GoSchema{
		GoType: "struct { Data map[string]DisputeInfo }",
		Properties: []Property{
			{
				GoName: "Data",
				Schema: GoSchema{
					GoType:                   "map[string]DisputeInfo",
					AdditionalPropertiesType: &disputeInfoSchema,
				},
			},
		},
	}

	result := schema.ValidateDecl("d", "typesValidator")
	expected := `
		var errors runtime.ValidationErrors
		for k, v := range d.Data {
			if validator, ok := any(v).(runtime.Validator); ok {
				if err := validator.Validate(); err != nil {
					errors = errors.Append(fmt.Sprintf("Data[%s]", k), err)
				}
			}
		}
		if len(errors) == 0 {
			return nil
		}
		return errors
	`
	assertCodeEqual(t, expected, result)
}
