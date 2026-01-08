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
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// RequestBodyDefinition describes a request body.
// Name is the name of the body.
// Required is whether the body is required.
// GoSchema is the GoSchema object describing the body.
// NameTag is the tag used to generate the type name,
// i.e. JSON, in which case we will produce "JSONBody".
// ContentType is the content type of the body.
// Default is whether this is the default body type.
// Encoding is the encoding options for formdata.
type RequestBodyDefinition struct {
	Name        string
	Required    bool
	Schema      GoSchema
	NameTag     string
	ContentType string
	Default     bool
	Encoding    map[string]RequestBodyEncoding
}

// TypeDef returns the Go type definition for a request body
func (r RequestBodyDefinition) TypeDef(opID string) TypeDefinition {
	return TypeDefinition{
		Name:   fmt.Sprintf("%s%sRequestBody", opID, r.NameTag),
		Schema: r.Schema,
	}
}

func (r RequestBodyDefinition) IsOptional() bool {
	return r.Schema.Constraints.Required == nil || !*r.Schema.Constraints.Required
}

// RequestBodyEncoding describes the encoding options for a request body.
// @see https://spec.openapis.org/oas/v3.1.0#fixed-fields-12
type RequestBodyEncoding struct {
	ContentType string
	Style       string
	Explode     *bool
}

// createBodyDefinition turns the OpenAPI body definitions into a list of our body definitions
// which will be used for code generation.
func createBodyDefinition(operationID string, body *v3high.RequestBody, options ParseOptions) (*RequestBodyDefinition, *TypeDefinition, error) {
	if body == nil {
		return nil, nil, nil
	}

	required := false
	if body.Required != nil {
		required = *body.Required
	}

	pair := body.Content.First()
	if pair == nil {
		return nil, nil, nil
	}

	contentType, content := pair.Key(), pair.Value()

	schemaProxy := content.Schema
	if schemaProxy == nil {
		return nil, nil, nil
	}

	var tag string
	var defaultBody bool

	switch {
	case contentType == "application/json":
		tag = "JSON"
		defaultBody = true
	case isMediaTypeJson(contentType):
		tag = mediaTypeToCamelCase(contentType)
	case strings.HasPrefix(contentType, "multipart/"):
		tag = "Multipart"
	case contentType == "application/x-www-form-urlencoded":
		tag = "Formdata"
	case contentType == "text/plain":
		tag = "Text"
	default:
		return nil, nil, nil
	}

	bodyTypeName := operationID + "Body"
	ref := schemaProxy.GoLow().GetReference()
	opts := options.WithReference(ref).WithPath([]string{bodyTypeName}).WithSpecLocation(SpecLocationBody)

	// For request bodies, filter out readOnly fields from the required list
	// since readOnly fields should only appear in responses, not requests
	hasReadOnlyRequired := filterReadOnlyFromRequired(schemaProxy)

	// If the schema has readOnly required fields and uses a $ref, we need to
	// generate a new struct instead of using a type alias, so we can have
	// different required fields for the request body vs the response
	optsForBody := opts
	if hasReadOnlyRequired && ref != "" {
		// Clear the reference so it generates a new struct instead of an alias
		optsForBody = opts.WithReference("")
	}

	bodySchema, err := GenerateGoSchema(schemaProxy, optsForBody)
	if err != nil {
		return nil, nil, fmt.Errorf("error generating request body definition: %w", err)
	}

	td := TypeDefinition{
		Name:             bodyTypeName,
		Schema:           bodySchema,
		SpecLocation:     SpecLocationBody,
		NeedsMarshaler:   needsMarshaler(bodySchema),
		HasSensitiveData: hasSensitiveData(bodySchema),
	}
	options.AddType(td)

	// If the request has a body, but it's not a user defined
	// type under #/components, we'll define a type for it, so
	// that we have an easy-to-use type for marshaling.
	if !bodySchema.DefineViaAlias {
		bodySchema.RefType = bodyTypeName
	}

	bodySchema.Constraints.Required = ptr(required)

	bd := &RequestBodyDefinition{
		Name:        bodyTypeName,
		Required:    required,
		Schema:      bodySchema,
		NameTag:     tag,
		ContentType: contentType,
		Default:     defaultBody,
	}

	if content.Encoding.Len() != 0 {
		bd.Encoding = make(map[string]RequestBodyEncoding)
		for k, v := range content.Encoding.FromOldest() {
			enc := RequestBodyEncoding{
				ContentType: v.ContentType,
				Style:       v.Style,
				Explode:     v.Explode,
			}
			bd.Encoding[k] = enc
		}
	}

	return bd, &td, nil
}

// filterReadOnlyFromRequired removes readOnly properties from the required list
// in request body schemas. ReadOnly properties should only be required in responses,
// not in requests. Returns true if any readOnly required fields were found and filtered.
func filterReadOnlyFromRequired(schemaProxy *base.SchemaProxy) bool {
	return filterReadOnlyFromRequiredWithVisited(schemaProxy, make(map[string]bool))
}

func filterReadOnlyFromRequiredWithVisited(schemaProxy *base.SchemaProxy, visited map[string]bool) bool {
	if schemaProxy == nil {
		return false
	}

	// Check for circular references
	ref := schemaProxy.GetReference()
	if ref != "" {
		if visited[ref] {
			return false
		}
		visited[ref] = true
	}

	schema := schemaProxy.Schema()
	if schema == nil {
		return false
	}

	hasReadOnlyRequired := false

	// Handle allOf, anyOf, oneOf schemas
	if schema.AllOf != nil {
		for _, subSchemaProxy := range schema.AllOf {
			if filterReadOnlyFromRequiredWithVisited(subSchemaProxy, visited) {
				hasReadOnlyRequired = true
			}
		}
	}

	if schema.AnyOf != nil {
		for _, subSchemaProxy := range schema.AnyOf {
			if filterReadOnlyFromRequiredWithVisited(subSchemaProxy, visited) {
				hasReadOnlyRequired = true
			}
		}
	}

	if schema.OneOf != nil {
		for _, subSchemaProxy := range schema.OneOf {
			if filterReadOnlyFromRequiredWithVisited(subSchemaProxy, visited) {
				hasReadOnlyRequired = true
			}
		}
	}

	// Handle regular object schemas with properties
	if schema.Properties != nil {
		// Build a list of readOnly property names
		readOnlyProps := make(map[string]bool)
		for propName, propProxy := range schema.Properties.FromOldest() {
			if propProxy == nil {
				continue
			}
			propSchema := propProxy.Schema()
			if propSchema != nil && propSchema.ReadOnly != nil && *propSchema.ReadOnly {
				readOnlyProps[propName] = true
			}
		}

		// Filter out readOnly properties from the required list
		if len(readOnlyProps) > 0 && len(schema.Required) > 0 {
			var filteredRequired []string
			for _, reqProp := range schema.Required {
				if !readOnlyProps[reqProp] {
					filteredRequired = append(filteredRequired, reqProp)
				} else {
					hasReadOnlyRequired = true
				}
			}
			schema.Required = filteredRequired
		}

		// Recursively filter nested object properties
		for _, propProxy := range schema.Properties.FromOldest() {
			if filterReadOnlyFromRequiredWithVisited(propProxy, visited) {
				hasReadOnlyRequired = true
			}
		}
	}

	return hasReadOnlyRequired
}
