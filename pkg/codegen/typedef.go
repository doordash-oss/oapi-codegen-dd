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
)

type SpecLocation string

const (
	SpecLocationPath     SpecLocation = "path"
	SpecLocationQuery    SpecLocation = "query"
	SpecLocationHeader   SpecLocation = "header"
	SpecLocationBody     SpecLocation = "body"
	SpecLocationResponse SpecLocation = "response"
	SpecLocationSchema   SpecLocation = "schema"
	SpecLocationUnion    SpecLocation = "union"
	SpecLocationWebhook  SpecLocation = "webhook"
)

// TypeDefinition describes a Go type definition in generated code.
// Name is the name of the type in the schema, eg, type <...> Person.
// JsonName is the name of the corresponding JSON description, as it will sometimes
// differ due to invalid characters.
// Schema is the GoSchema object used to populate the type description.
// SpecLocation indicates where in the OpenAPI spec this type was defined.
// NeedsMarshaler indicates whether this type needs a custom marshaler/unmarshaler.
// HasSensitiveData indicates whether this type has any properties marked as sensitive.
type TypeDefinition struct {
	Name             string
	JsonName         string
	Schema           GoSchema
	SpecLocation     SpecLocation
	NeedsMarshaler   bool
	HasSensitiveData bool
}

func (t TypeDefinition) IsAlias() bool {
	return t.Schema.DefineViaAlias
}

func (t TypeDefinition) IsOptional() bool {
	return t.Schema.Constraints.Required == nil || !*t.Schema.Constraints.Required
}

// GetErrorResponse generates a Go code snippet that returns an error response
// based on the predefined spec error path.
// The path supports array access with [] suffix, e.g., "data[].message[]" will
// access the first element of each array. A path that continues inside a oneOf/anyOf
// union switches on the decoded variant.
func (t TypeDefinition) GetErrorResponse(errTypes map[string]string, alias string, typeSchemaMap map[string]GoSchema) string {
	fields, err := resolveErrorPath(t.Name, errTypes, t.Schema, typeSchemaMap)
	if err != nil {
		return unknownErrorReturn
	}

	varIndex := 0
	return strings.Join(errorResponseCode(fields, alias, &varIndex), "\n")
}

// GetErrorConstructor generates a Go constructor function for an error type.
// It creates nested struct literals based on the error-mapping path.
// If no error-mapping is configured, it returns a simple constructor.
func (t TypeDefinition) GetErrorConstructor(errTypes map[string]string, typeSchemaMap map[string]GoSchema) string {
	fields, err := resolveErrorPath(t.Name, errTypes, t.Schema, typeSchemaMap)

	// No error-mapping or invalid path - return empty (template only calls this when error-mapping exists).
	// A path into union variants has no single shape to build from a message.
	if err != nil || fields[len(fields)-1].union != nil {
		return ""
	}

	return errorConstructorCode(t.Name, fields)
}
