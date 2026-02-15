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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTypeDefinition_GetErrorResponse(t *testing.T) {
	t.Run("single property without pointer", func(t *testing.T) {
		typ := TypeDefinition{
			Name: "ResError",
			Schema: GoSchema{
				Properties: []Property{
					{
						GoName:        "ID",
						JsonFieldName: "id",
					},
					{
						GoName:        "Details",
						JsonFieldName: "details",
						Schema: GoSchema{
							GoType: "string",
						},
					},
				},
			},
		}
		res := typ.GetErrorResponse(map[string]string{"ResError": "details"}, "e", map[string]GoSchema{})
		expected := `res0 := e.Details
return res0`
		assert.Equal(t, expected, res)
	})

	t.Run("single property with pointer", func(t *testing.T) {
		typ := TypeDefinition{
			Name: "ResError",
			Schema: GoSchema{
				Properties: []Property{
					{
						GoName:        "ID",
						JsonFieldName: "id",
					},
					{
						GoName:        "Details",
						JsonFieldName: "details",
						Schema: GoSchema{
							GoType: "string",
						},
						Constraints: Constraints{
							Nullable: ptr(true),
						},
					},
				},
			},
		}
		res := typ.GetErrorResponse(map[string]string{"ResError": "details"}, "e", map[string]GoSchema{})
		expected := `res0 := e.Details
if res0 == nil { return "unknown error" }
res1 := *res0
return res1`
		assert.Equal(t, expected, res)
	})

	t.Run("property with name error", func(t *testing.T) {
		typ := TypeDefinition{
			Name: "ResError",
			Schema: GoSchema{
				Properties: []Property{
					{
						GoName:        "ErrorData",
						JsonFieldName: "error",
						Schema: GoSchema{
							Properties: []Property{
								{
									GoName:        "Message",
									JsonFieldName: "message",
									Schema: GoSchema{
										GoType: "string",
									},
								},
							},
						},
					},
				},
			},
		}
		res := typ.GetErrorResponse(map[string]string{"ResError": "error.message"}, "e", map[string]GoSchema{})
		expected := `res0 := e.ErrorData
res1 := res0.Message
return res1`
		assert.Equal(t, expected, res)
	})

	t.Run("nested property without pointer", func(t *testing.T) {
		typ := TypeDefinition{
			Name: "ResError",
			Schema: GoSchema{
				Properties: []Property{
					{
						GoName:        "Data",
						JsonFieldName: "data",
						Schema: GoSchema{
							Properties: []Property{
								{
									GoName:        "Details",
									JsonFieldName: "details",
									Schema: GoSchema{
										Properties: []Property{
											{
												GoName:        "Message",
												JsonFieldName: "message",
												Schema:        GoSchema{GoType: "string"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		res := typ.GetErrorResponse(map[string]string{"ResError": "data.details.message"}, "e", map[string]GoSchema{})
		expected := `res0 := e.Data
res1 := res0.Details
res2 := res1.Message
return res2`
		assert.Equal(t, expected, res)
	})

	t.Run("nested property with pointer", func(t *testing.T) {
		typ := TypeDefinition{
			Name: "ResError",
			Schema: GoSchema{
				Properties: []Property{
					{
						GoName:        "Data",
						JsonFieldName: "data",
						Constraints:   Constraints{Nullable: ptr(true)},
						Schema: GoSchema{
							Properties: []Property{
								{
									GoName:        "Details",
									JsonFieldName: "details",
									Constraints:   Constraints{Nullable: ptr(true)},
									Schema: GoSchema{
										Properties: []Property{
											{
												GoName:        "Message",
												JsonFieldName: "message",
												Constraints:   Constraints{Nullable: ptr(true)},
												Schema: GoSchema{
													GoType: "string",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		res := typ.GetErrorResponse(map[string]string{"ResError": "data.details.message"}, "e", map[string]GoSchema{})
		expected := `res0 := e.Data
if res0 == nil { return "unknown error" }
res1 := *res0
res2 := res1.Details
if res2 == nil { return "unknown error" }
res3 := *res2
res4 := res3.Message
if res4 == nil { return "unknown error" }
res5 := *res4
return res5`
		assert.Equal(t, expected, res)
	})

	t.Run("property referencing another type", func(t *testing.T) {
		// Define the referenced type
		errorDataType := TypeDefinition{
			Name: "ErrorData",
			Schema: GoSchema{
				Properties: []Property{
					{
						GoName:        "Message",
						JsonFieldName: "message",
						Schema: GoSchema{
							GoType: "string",
						},
					},
				},
			},
		}

		// Define the main type that references ErrorData
		typ := TypeDefinition{
			Name: "InvalidRequestError",
			Schema: GoSchema{
				Properties: []Property{
					{
						GoName:        "ErrorData",
						JsonFieldName: "error",
						Schema: GoSchema{
							GoType: "ErrorData",
						},
						Constraints: Constraints{
							Nullable: boolPtr(true),
						},
					},
				},
			},
		}

		// Build a type schema map so the function can resolve the reference
		typeSchemaMap := map[string]GoSchema{
			"InvalidRequestError": typ.Schema,
			"ErrorData":           errorDataType.Schema,
		}
		res := typ.GetErrorResponse(map[string]string{"InvalidRequestError": "error.message"}, "r", typeSchemaMap)
		expected := `res0 := r.ErrorData
if res0 == nil { return "unknown error" }
res1 := *res0
res2 := res1.Message
return res2`
		assert.Equal(t, expected, res)
	})

	t.Run("array property with bracket notation", func(t *testing.T) {
		typ := TypeDefinition{
			Name: "ServiceError",
			Schema: GoSchema{
				Properties: []Property{
					{
						GoName:        "Data",
						JsonFieldName: "data",
						Schema: GoSchema{
							ArrayType: &GoSchema{
								GoType: "ErrorData",
							},
						},
					},
				},
			},
		}

		errorDataSchema := GoSchema{
			Properties: []Property{
				{
					GoName:        "Message",
					JsonFieldName: "message",
					Schema: GoSchema{
						ArrayType: &GoSchema{
							GoType: "string",
						},
					},
				},
			},
		}

		typeSchemaMap := map[string]GoSchema{
			"ServiceError": typ.Schema,
			"ErrorData":    errorDataSchema,
		}

		res := typ.GetErrorResponse(map[string]string{"ServiceError": "data[].message[]"}, "s", typeSchemaMap)
		expected := `res0 := s.Data
if len(res0) == 0 { return "unknown error" }
res1 := res0[0]
res2 := res1.Message
if len(res2) == 0 { return "unknown error" }
res3 := res2[0]
return res3`
		assert.Equal(t, expected, res)
	})

	t.Run("array property without bracket notation returns array", func(t *testing.T) {
		typ := TypeDefinition{
			Name: "ServiceError",
			Schema: GoSchema{
				Properties: []Property{
					{
						GoName:        "Messages",
						JsonFieldName: "messages",
						Schema: GoSchema{
							ArrayType: &GoSchema{
								GoType: "string",
							},
						},
					},
				},
			},
		}

		res := typ.GetErrorResponse(map[string]string{"ServiceError": "messages"}, "s", map[string]GoSchema{})
		expected := `res0 := s.Messages
return res0`
		assert.Equal(t, expected, res)
	})

	t.Run("nullable array property with bracket notation", func(t *testing.T) {
		// Arrays are represented as slices []T, not pointers *[]T,
		// so we don't need nil check + dereference, just len check
		typ := TypeDefinition{
			Name: "ServiceError",
			Schema: GoSchema{
				Properties: []Property{
					{
						GoName:        "Errors",
						JsonFieldName: "errors",
						Constraints:   Constraints{Nullable: ptr(true)},
						Schema: GoSchema{
							ArrayType: &GoSchema{
								GoType: "string",
							},
						},
					},
				},
			},
		}

		res := typ.GetErrorResponse(map[string]string{"ServiceError": "errors[]"}, "s", map[string]GoSchema{})
		expected := `res0 := s.Errors
if len(res0) == 0 { return "unknown error" }
res1 := res0[0]
return res1`
		assert.Equal(t, expected, res)
	})
}

func TestTypeDefinition_GetErrorConstructor(t *testing.T) {
	t.Run("no error mapping - returns empty", func(t *testing.T) {
		typ := TypeDefinition{
			Name:   "BadRequestError",
			Schema: GoSchema{},
		}
		res := typ.GetErrorConstructor(map[string]string{}, map[string]GoSchema{})
		assert.Equal(t, "", res)
	})

	t.Run("single property without pointer", func(t *testing.T) {
		typ := TypeDefinition{
			Name: "ResError",
			Schema: GoSchema{
				Properties: []Property{
					{
						GoName:        "Details",
						JsonFieldName: "details",
						Schema: GoSchema{
							GoType: "string",
						},
					},
				},
			},
		}
		res := typ.GetErrorConstructor(map[string]string{"ResError": "details"}, map[string]GoSchema{})
		expected := `func NewResError(message string) ResError {
	return ResError{Details: message}
}`
		assert.Equal(t, expected, res)
	})

	t.Run("nested property with pointer", func(t *testing.T) {
		// Define the referenced type
		errorDataType := TypeDefinition{
			Name: "ErrorData",
			Schema: GoSchema{
				Properties: []Property{
					{
						GoName:        "Message",
						JsonFieldName: "message",
						Schema: GoSchema{
							GoType: "string",
						},
					},
				},
			},
		}

		// Define the main type that references ErrorData
		typ := TypeDefinition{
			Name: "InvalidRequestError",
			Schema: GoSchema{
				Properties: []Property{
					{
						GoName:        "ErrorData",
						JsonFieldName: "error",
						Schema: GoSchema{
							GoType: "ErrorData",
						},
						Constraints: Constraints{
							Nullable: boolPtr(true),
						},
					},
				},
			},
		}

		typeSchemaMap := map[string]GoSchema{
			"InvalidRequestError": typ.Schema,
			"ErrorData":           errorDataType.Schema,
		}
		res := typ.GetErrorConstructor(map[string]string{"InvalidRequestError": "error.message"}, typeSchemaMap)
		expected := `func NewInvalidRequestError(message string) InvalidRequestError {
	return InvalidRequestError{ErrorData: &ErrorData{Message: message}}
}`
		assert.Equal(t, expected, res)
	})

	t.Run("array property with bracket notation", func(t *testing.T) {
		typ := TypeDefinition{
			Name: "ServiceError",
			Schema: GoSchema{
				Properties: []Property{
					{
						GoName:        "Data",
						JsonFieldName: "data",
						Schema: GoSchema{
							ArrayType: &GoSchema{
								GoType: "ErrorData",
							},
						},
					},
				},
			},
		}

		errorDataSchema := GoSchema{
			Properties: []Property{
				{
					GoName:        "Message",
					JsonFieldName: "message",
					Schema: GoSchema{
						GoType: "string",
					},
				},
			},
		}

		typeSchemaMap := map[string]GoSchema{
			"ServiceError": typ.Schema,
			"ErrorData":    errorDataSchema,
		}

		res := typ.GetErrorConstructor(map[string]string{"ServiceError": "data[].message"}, typeSchemaMap)
		expected := `func NewServiceError(message string) ServiceError {
	return ServiceError{Data: []ErrorData{{Message: message}}}
}`
		assert.Equal(t, expected, res)
	})

	t.Run("nested property with nullable primitive", func(t *testing.T) {
		// Define the referenced type with nullable string
		errorDetailsType := TypeDefinition{
			Name: "ErrorDetails",
			Schema: GoSchema{
				Properties: []Property{
					{
						GoName:        "Message",
						JsonFieldName: "message",
						Schema: GoSchema{
							GoType: "string",
						},
						Constraints: Constraints{
							Nullable: boolPtr(true),
						},
					},
				},
			},
		}

		// Define the main type that references ErrorDetails
		typ := TypeDefinition{
			Name: "ServiceError",
			Schema: GoSchema{
				Properties: []Property{
					{
						GoName:        "ErrorData",
						JsonFieldName: "error",
						Schema: GoSchema{
							GoType: "ErrorDetails",
						},
						Constraints: Constraints{
							Nullable: boolPtr(true),
						},
					},
				},
			},
		}

		typeSchemaMap := map[string]GoSchema{
			"ServiceError": typ.Schema,
			"ErrorDetails": errorDetailsType.Schema,
		}
		res := typ.GetErrorConstructor(map[string]string{"ServiceError": "error.message"}, typeSchemaMap)
		expected := `func NewServiceError(message string) ServiceError {
	return ServiceError{ErrorData: &ErrorDetails{Message: runtime.Ptr(message)}}
}`
		assert.Equal(t, expected, res)
	})
}

func boolPtr(b bool) *bool {
	return &b
}
