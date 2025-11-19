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
	"net/http"
	"strings"
)

// OperationDefinition describes an Operation.
// ID The operation_id description from Swagger, used to generate function names.
// Summary string from OpenAPI spec, used to generate a comment.
// Description string from OpenAPI spec.
// Method The HTTP method for this operation.
// Path The path for this operation.
// PathParams Parameters in the path
// Header HTTP headers.
// Query Query
// TypeDefinitions These are all the types we need to define for this operation.
// BodyRequired Whether the body is required for this operation.
type OperationDefinition struct {
	ID          string
	Summary     string
	Description string
	Method      string
	Path        string
	PathParams  *TypeDefinition
	Header      *TypeDefinition
	Query       *RequestParametersDefinition

	TypeDefinitions []TypeDefinition
	// TODO: check if can be removed
	BodyRequired bool

	Body     *RequestBodyDefinition
	Response ResponseDefinition
}

// RequiresParamObject indicates If we have parameters other than path parameters, they're bundled into an
// object. Returns true if we have any of those.
// This is used from the template engine.
func (o OperationDefinition) RequiresParamObject() bool {
	return o.Query != nil || o.Header != nil
}

// SummaryAsComment returns the Operations summary as a multi line comment
func (o OperationDefinition) SummaryAsComment() string {
	if o.Summary == "" {
		return ""
	}
	trimmed := strings.TrimSuffix(o.Summary, "\n")
	parts := strings.Split(trimmed, "\n")
	for i, p := range parts {
		parts[i] = "// " + p
	}
	return strings.Join(parts, "\n")
}

func (o OperationDefinition) GetSuccessResponse() string {
	if o.Response.SuccessStatusCode == http.StatusNoContent {
		return ""
	}
	return o.Response.Success.ResponseName
}

func (o OperationDefinition) HasRequestOptions() bool {
	return o.PathParams != nil || o.Header != nil || o.Query != nil || o.Body != nil
}

// filterParameterDefinitionByType returns the subset of the specified parameters which are of the
// specified type.
func filterParameterDefinitionByType(params []ParameterDefinition, in string) []ParameterDefinition {
	var out []ParameterDefinition
	for _, p := range params {
		if p.In == in {
			out = append(out, p)
		}
	}
	return out
}

// createOperationID generates a unique operation ID based on the HTTP method and path.
// If the initial value is provided, it will be used.
// The resulting operation ID is a camel-cased string.
func createOperationID(method, path, initial string) (string, error) {
	if initial != "" {
		return typeNamePrefix(initial) + nameNormalizer(initial), nil
	}

	if method == "" {
		return "", ErrOperationNameEmpty
	}

	if path == "" {
		return "", ErrRequestPathEmpty
	}

	res := strings.ToLower(method)

	for _, part := range strings.Split(path, "/") {
		if part != "" {
			res = res + "-" + part
		}
	}

	return nameNormalizer(res), nil
}
