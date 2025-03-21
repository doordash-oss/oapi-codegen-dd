// Copyright 2019 DeepMap, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package codegen

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/doordash/oapi-codegen/v2/pkg/util"
)

// OperationDefinition describes an Operation
type OperationDefinition struct {
	OperationId string // The operation_id description from Swagger, used to generate function names

	PathParams      []ParameterDefinition // Parameters in the path, eg, /path/:param
	HeaderParams    []ParameterDefinition // Parameters in HTTP headers
	QueryParams     []ParameterDefinition // Parameters in the query, /path?param
	TypeDefinitions []TypeDefinition      // These are all the types we need to define for this operation
	BodyRequired    bool
	Bodies          []RequestBodyDefinition // The list of bodies for which to generate handlers.
	Responses       []ResponseDefinition    // The list of responses that can be accepted by handlers.
	Summary         string                  // Summary string from Swagger, used to generate a comment
	Method          string                  // GET, POST, DELETE, etc.
	Path            string                  // The Swagger path for the operation, like /resource/{id}
	Spec            *openapi3.Operation
}

// Params returns the list of all parameters except Path parameters. Path parameters
// are handled differently from the rest, since they're mandatory.
func (o *OperationDefinition) Params() []ParameterDefinition {
	result := append(o.QueryParams, o.HeaderParams...)
	return result
}

// AllParams returns all parameters
func (o *OperationDefinition) AllParams() []ParameterDefinition {
	result := append(o.QueryParams, o.HeaderParams...)
	result = append(result, o.PathParams...)
	return result
}

// If we have parameters other than path parameters, they're bundled into an
// object. Returns true if we have any of those. This is used from the template
// engine.
func (o *OperationDefinition) RequiresParamObject() bool {
	return len(o.Params()) > 0
}

// HasBody is called by the template engine to determine whether to generate body
// marshaling code on the client. This is true for all body types, whether
// we generate types for them.
func (o *OperationDefinition) HasBody() bool {
	return o.Spec.RequestBody != nil
}

// SummaryAsComment returns the Operations summary as a multi line comment
func (o *OperationDefinition) SummaryAsComment() string {
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

// GetResponseTypeDefinitions produces a list of type definitions for a given Operation for the response
// types which we know how to parse. These will be turned into fields on a
// response object for automatic deserialization of responses in the generated
// Client code. See "client-with-responses.tmpl".
func (o *OperationDefinition) GetResponseTypeDefinitions() ([]ResponseTypeDefinition, error) {
	var tds []ResponseTypeDefinition

	if o.Spec == nil || o.Spec.Responses == nil {
		return tds, nil
	}

	sortedResponsesKeys := SortedMapKeys(o.Spec.Responses.Map())
	for _, responseName := range sortedResponsesKeys {
		responseRef := o.Spec.Responses.Value(responseName)

		// We can only generate a type if we have a value:
		if responseRef.Value != nil {
			jsonCount := 0
			for mediaType := range responseRef.Value.Content {
				if util.IsMediaTypeJson(mediaType) {
					jsonCount++
				}
			}

			sortedContentKeys := SortedMapKeys(responseRef.Value.Content)
			for _, contentTypeName := range sortedContentKeys {
				contentType := responseRef.Value.Content[contentTypeName]
				// We can only generate a type if we have a schema:
				if contentType.Schema != nil {
					responseSchema, err := GenerateGoSchema(contentType.Schema, []string{o.OperationId, responseName})
					if err != nil {
						return nil, fmt.Errorf("Unable to determine Go type for %s.%s: %w", o.OperationId, contentTypeName, err)
					}

					var typeName string
					switch {

					// HAL+JSON:
					case StringInArray(contentTypeName, contentTypesHalJSON):
						typeName = fmt.Sprintf("HALJSON%s", nameNormalizer(responseName))
					case "application/json" == contentTypeName:
						// if it's the standard application/json
						typeName = fmt.Sprintf("JSON%s", nameNormalizer(responseName))
					// Vendored JSON
					case StringInArray(contentTypeName, contentTypesJSON) || util.IsMediaTypeJson(contentTypeName):
						baseTypeName := fmt.Sprintf("%s%s", nameNormalizer(contentTypeName), nameNormalizer(responseName))

						typeName = strings.ReplaceAll(baseTypeName, "Json", "JSON")
					// YAML:
					case StringInArray(contentTypeName, contentTypesYAML):
						typeName = fmt.Sprintf("YAML%s", nameNormalizer(responseName))
					// XML:
					case StringInArray(contentTypeName, contentTypesXML):
						typeName = fmt.Sprintf("XML%s", nameNormalizer(responseName))
					default:
						continue
					}

					td := ResponseTypeDefinition{
						TypeDefinition: TypeDefinition{
							TypeName: typeName,
							Schema:   responseSchema,
						},
						ResponseName:              responseName,
						ContentTypeName:           contentTypeName,
						AdditionalTypeDefinitions: responseSchema.GetAdditionalTypeDefs(),
					}
					if IsGoTypeReference(responseRef.Ref) {
						refType, err := RefPathToGoType(responseRef.Ref)
						if err != nil {
							return nil, fmt.Errorf("error dereferencing response Ref: %w", err)
						}
						if jsonCount > 1 && util.IsMediaTypeJson(contentTypeName) {
							refType += mediaTypeToCamelCase(contentTypeName)
						}
						td.Schema.RefType = refType
					}
					tds = append(tds, td)
				}
			}
		}
	}
	return tds, nil
}

func (o OperationDefinition) HasMaskedRequestContentTypes() bool {
	for _, body := range o.Bodies {
		if !body.IsFixedContentType() {
			return true
		}
	}
	return false
}

type ResponseDefinition struct {
	StatusCode  string
	Description string
	Contents    []ResponseContentDefinition
	Headers     []ResponseHeaderDefinition
	Ref         string
}

func (r ResponseDefinition) HasFixedStatusCode() bool {
	_, err := strconv.Atoi(r.StatusCode)
	return err == nil
}

func (r ResponseDefinition) GoName() string {
	return SchemaNameToTypeName(r.StatusCode)
}

func (r ResponseDefinition) IsRef() bool {
	return r.Ref != ""
}

func (r ResponseDefinition) IsExternalRef() bool {
	if !r.IsRef() {
		return false
	}
	return strings.Contains(r.Ref, ".")
}

type ResponseContentDefinition struct {
	// This is the schema describing this content
	Schema Schema

	// This is the content type corresponding to the body, eg, application/json
	ContentType string

	// When we generate type names, we need a Tag for it, such as JSON, in
	// which case we will produce "Response200JSONContent".
	NameTag string
}

// TypeDef returns the Go type definition for a request body
func (r ResponseContentDefinition) TypeDef(opID string, statusCode int) *TypeDefinition {
	return &TypeDefinition{
		TypeName: fmt.Sprintf("%s%v%sResponse", opID, statusCode, r.NameTagOrContentType()),
		Schema:   r.Schema,
	}
}

func (r ResponseContentDefinition) IsSupported() bool {
	return r.NameTag != ""
}

// HasFixedContentType returns true if content type has fixed content type, i.e. contains no "*" symbol
func (r ResponseContentDefinition) HasFixedContentType() bool {
	return !strings.Contains(r.ContentType, "*")
}

func (r ResponseContentDefinition) NameTagOrContentType() string {
	if r.NameTag != "" {
		return r.NameTag
	}
	return SchemaNameToTypeName(r.ContentType)
}

// IsJSON returns whether this is a JSON media type, for instance:
// - application/json
// - application/vnd.api+json
// - application/*+json
func (r ResponseContentDefinition) IsJSON() bool {
	return util.IsMediaTypeJson(r.ContentType)
}

type ResponseHeaderDefinition struct {
	Name   string
	GoName string
	Schema Schema
}

// FilterParameterDefinitionByType returns the subset of the specified parameters which are of the
// specified type.
func FilterParameterDefinitionByType(params []ParameterDefinition, in string) []ParameterDefinition {
	var out []ParameterDefinition
	for _, p := range params {
		if p.In == in {
			out = append(out, p)
		}
	}
	return out
}

// OperationDefinitions returns all operations for a swagger definition.
func OperationDefinitions(swagger *openapi3.T, initialismOverrides bool) ([]OperationDefinition, error) {
	var operations []OperationDefinition

	var toCamelCaseFunc func(string) string
	if initialismOverrides {
		toCamelCaseFunc = ToCamelCaseWithInitialism
	} else {
		toCamelCaseFunc = ToCamelCase
	}

	if swagger == nil || swagger.Paths == nil {
		return operations, nil
	}

	for _, requestPath := range SortedMapKeys(swagger.Paths.Map()) {
		pathItem := swagger.Paths.Value(requestPath)
		// These are parameters defined for all methods on a given path. They
		// are shared by all methods.
		globalParams, err := DescribeParameters(pathItem.Parameters, nil)
		if err != nil {
			return nil, fmt.Errorf("error describing global parameters for %s: %s",
				requestPath, err)
		}

		// Each path can have a number of operations, POST, GET, OPTIONS, etc.
		pathOps := pathItem.Operations()
		for _, opName := range SortedMapKeys(pathOps) {
			op := pathOps[opName]
			if pathItem.Servers != nil {
				op.Servers = &pathItem.Servers
			}
			// We rely on OperationID to generate function names, it's required
			if op.OperationID == "" {
				op.OperationID, err = generateDefaultOperationID(opName, requestPath, toCamelCaseFunc)
				if err != nil {
					return nil, fmt.Errorf("error generating default OperationID for %s/%s: %s",
						opName, requestPath, err)
				}
			} else {
				op.OperationID = nameNormalizer(op.OperationID)
			}
			op.OperationID = typeNamePrefix(op.OperationID) + op.OperationID

			// These are parameters defined for the specific path method that
			// we're iterating over.
			localParams, err := DescribeParameters(op.Parameters, []string{op.OperationID + "Params"})
			if err != nil {
				return nil, fmt.Errorf("error describing global parameters for %s/%s: %s",
					opName, requestPath, err)
			}
			// All the parameters required by a handler are the union of the
			// global parameters and the local parameters.
			allParams, err := CombineOperationParameters(globalParams, localParams)
			if err != nil {
				return nil, err
			}

			// Order the path parameters to match the order as specified in
			// the path, not in the swagger spec, and validate that the parameter
			// names match, as downstream code depends on that.
			pathParams := FilterParameterDefinitionByType(allParams, "path")
			pathParams, err = SortParamsByPath(requestPath, pathParams)
			if err != nil {
				return nil, err
			}

			bodyDefinitions, typeDefinitions, err := GenerateBodyDefinitions(op.OperationID, op.RequestBody)
			if err != nil {
				return nil, fmt.Errorf("error generating body definitions: %w", err)
			}

			responseDefinitions, err := GenerateResponseDefinitions(op.OperationID, op.Responses.Map())
			if err != nil {
				return nil, fmt.Errorf("error generating response definitions: %w", err)
			}

			opDef := OperationDefinition{
				PathParams:   pathParams,
				HeaderParams: FilterParameterDefinitionByType(allParams, "header"),
				QueryParams:  FilterParameterDefinitionByType(allParams, "query"),
				OperationId:  nameNormalizer(op.OperationID),
				// Replace newlines in summary.
				Summary:         op.Summary,
				Method:          opName,
				Path:            requestPath,
				Spec:            op,
				Bodies:          bodyDefinitions,
				Responses:       responseDefinitions,
				TypeDefinitions: typeDefinitions,
			}

			if op.RequestBody != nil {
				opDef.BodyRequired = op.RequestBody.Value.Required
			}

			// Generate all the type definitions needed for this operation
			opDef.TypeDefinitions = append(opDef.TypeDefinitions, GenerateTypeDefsForOperation(opDef)...)

			operations = append(operations, opDef)
		}
	}
	return operations, nil
}

func generateDefaultOperationID(opName string, requestPath string, toCamelCaseFunc func(string) string) (string, error) {
	var operationId = strings.ToLower(opName)

	if opName == "" {
		return "", fmt.Errorf("operation name cannot be an empty string")
	}

	if requestPath == "" {
		return "", fmt.Errorf("request path cannot be an empty string")
	}

	for _, part := range strings.Split(requestPath, "/") {
		if part != "" {
			operationId = operationId + "-" + part
		}
	}

	return nameNormalizer(operationId), nil
}

func GenerateResponseDefinitions(operationID string, responses map[string]*openapi3.ResponseRef) ([]ResponseDefinition, error) {
	var responseDefinitions []ResponseDefinition
	// do not let multiple status codes ref to same response, it will break the type switch
	refSet := make(map[string]struct{})

	for _, statusCode := range SortedMapKeys(responses) {
		responseOrRef := responses[statusCode]
		if responseOrRef == nil {
			continue
		}
		response := responseOrRef.Value

		var responseContentDefinitions []ResponseContentDefinition

		for _, contentType := range SortedMapKeys(response.Content) {
			content := response.Content[contentType]
			var tag string
			switch {
			case contentType == "application/json":
				tag = "JSON"
			case util.IsMediaTypeJson(contentType):
				tag = mediaTypeToCamelCase(contentType)
			case contentType == "application/x-www-form-urlencoded":
				tag = "Formdata"
			case strings.HasPrefix(contentType, "multipart/"):
				tag = "Multipart"
			case contentType == "text/plain":
				tag = "Text"
			default:
				rcd := ResponseContentDefinition{
					ContentType: contentType,
				}
				responseContentDefinitions = append(responseContentDefinitions, rcd)
				continue
			}

			responseTypeName := operationID + statusCode + tag + "Response"
			contentSchema, err := GenerateGoSchema(content.Schema, []string{responseTypeName})
			if err != nil {
				return nil, fmt.Errorf("error generating request body definition: %w", err)
			}

			rcd := ResponseContentDefinition{
				ContentType: contentType,
				NameTag:     tag,
				Schema:      contentSchema,
			}

			responseContentDefinitions = append(responseContentDefinitions, rcd)
		}

		var responseHeaderDefinitions []ResponseHeaderDefinition
		for _, headerName := range SortedMapKeys(response.Headers) {
			header := response.Headers[headerName]
			contentSchema, err := GenerateGoSchema(header.Value.Schema, []string{})
			if err != nil {
				return nil, fmt.Errorf("error generating response header definition: %w", err)
			}
			headerDefinition := ResponseHeaderDefinition{Name: headerName, GoName: SchemaNameToTypeName(headerName), Schema: contentSchema}
			responseHeaderDefinitions = append(responseHeaderDefinitions, headerDefinition)
		}

		rd := ResponseDefinition{
			StatusCode: statusCode,
			Contents:   responseContentDefinitions,
			Headers:    responseHeaderDefinitions,
		}
		if response.Description != nil {
			rd.Description = *response.Description
		}
		if IsGoTypeReference(responseOrRef.Ref) {
			// Convert the reference path to Go type
			refType, err := RefPathToGoType(responseOrRef.Ref)
			if err != nil {
				return nil, fmt.Errorf("error turning reference (%s) into a Go type: %w", responseOrRef.Ref, err)
			}
			// Check if this ref is already used by another response definition. If not use the ref
			// If we let multiple response definitions alias to same response it will break the type switch
			// so only the first response will use the ref, other will generate new structs
			if _, ok := refSet[refType]; !ok {
				rd.Ref = refType
				refSet[refType] = struct{}{}
			}
		}
		responseDefinitions = append(responseDefinitions, rd)
	}

	return responseDefinitions, nil
}

func GenerateTypeDefsForOperation(op OperationDefinition) []TypeDefinition {
	var typeDefs []TypeDefinition
	// Start with the params object itself
	if len(op.Params()) != 0 {
		typeDefs = append(typeDefs, GenerateParamsTypes(op)...)
	}

	// Now, go through all the additional types we need to declare.
	for _, param := range op.AllParams() {
		typeDefs = append(typeDefs, param.Schema.AdditionalTypes...)
	}

	for _, body := range op.Bodies {
		typeDefs = append(typeDefs, body.Schema.AdditionalTypes...)
	}
	return typeDefs
}

// GenerateParamsTypes defines the schema for a parameters definition object
// which encapsulates all the query, header and cookie parameters for an operation.
func GenerateParamsTypes(op OperationDefinition) []TypeDefinition {
	var typeDefs []TypeDefinition

	objectParams := op.QueryParams
	objectParams = append(objectParams, op.HeaderParams...)

	typeName := op.OperationId + "Params"

	s := Schema{}
	for _, param := range objectParams {
		pSchema := param.Schema
		param.Style()
		if pSchema.HasAdditionalProperties {
			propRefName := strings.Join([]string{typeName, param.GoName()}, "_")
			pSchema.RefType = propRefName
			typeDefs = append(typeDefs, TypeDefinition{
				TypeName: propRefName,
				Schema:   param.Schema,
			})
		}
		prop := Property{
			Description:   param.Spec.Description,
			JsonFieldName: param.ParamName,
			Required:      param.Required,
			Schema:        pSchema,
			NeedsFormTag:  param.Style() == "form",
			Extensions:    param.Spec.Extensions,
		}
		s.Properties = append(s.Properties, prop)
	}

	s.Description = op.Spec.Description
	s.GoType = GenStructFromSchema(s)

	td := TypeDefinition{
		TypeName: typeName,
		Schema:   s,
	}
	return append(typeDefs, td)
}

// GenerateTypesForOperations generates code for all types produced within operations
func GenerateTypesForOperations(t *template.Template, ops []OperationDefinition) (string, error) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	addTypes, err := GenerateTemplates([]string{"param-types.tmpl", "request-bodies.tmpl"}, t, ops)
	if err != nil {
		return "", fmt.Errorf("error generating type boilerplate for operations: %w", err)
	}
	if _, err := w.WriteString(addTypes); err != nil {
		return "", fmt.Errorf("error writing boilerplate to buffer: %w", err)
	}

	// Generate boiler plate for all additional types.
	var td []TypeDefinition
	for _, op := range ops {
		td = append(td, op.TypeDefinitions...)
	}

	addProps, err := GenerateAdditionalPropertyBoilerplate(t, td)
	if err != nil {
		return "", fmt.Errorf("error generating additional properties boilerplate for operations: %w", err)
	}

	if _, err := w.WriteString("\n"); err != nil {
		return "", fmt.Errorf("error generating additional properties boilerplate for operations: %w", err)
	}

	if _, err := w.WriteString(addProps); err != nil {
		return "", fmt.Errorf("error generating additional properties boilerplate for operations: %w", err)
	}

	if err = w.Flush(); err != nil {
		return "", fmt.Errorf("error flushing output buffer for server interface: %w", err)
	}

	return buf.String(), nil
}

// GenerateClient uses the template engine to generate the function which registers our wrappers
// as Echo path handlers.
func GenerateClient(t *template.Template, ops []OperationDefinition) (string, error) {
	return GenerateTemplates([]string{"client.tmpl"}, t, ops)
}

// GenerateClientWithResponses generates a client which extends the basic client which does response
// unmarshaling.
func GenerateClientWithResponses(t *template.Template, ops []OperationDefinition) (string, error) {
	return GenerateTemplates([]string{"client-with-responses.tmpl"}, t, ops)
}

// GenerateTemplates used to generate templates
func GenerateTemplates(templates []string, t *template.Template, ops interface{}) (string, error) {
	var generatedTemplates []string
	for _, tmpl := range templates {
		var buf bytes.Buffer
		w := bufio.NewWriter(&buf)

		if err := t.ExecuteTemplate(w, tmpl, ops); err != nil {
			return "", fmt.Errorf("error generating %s: %s", tmpl, err)
		}
		if err := w.Flush(); err != nil {
			return "", fmt.Errorf("error flushing output buffer for %s: %s", tmpl, err)
		}
		generatedTemplates = append(generatedTemplates, buf.String())
	}

	return strings.Join(generatedTemplates, "\n"), nil
}

// CombineOperationParameters combines the Parameters defined at a global level (Parameters defined for all methods on a given path) with the Parameters defined at a local level (Parameters defined for a specific path), preferring the locally defined parameter over the global one
func CombineOperationParameters(globalParams []ParameterDefinition, localParams []ParameterDefinition) ([]ParameterDefinition, error) {
	allParams := make([]ParameterDefinition, 0, len(globalParams)+len(localParams))
	dupCheck := make(map[string]map[string]string)
	for _, p := range localParams {
		if dupCheck[p.In] == nil {
			dupCheck[p.In] = make(map[string]string)
		}
		if _, exist := dupCheck[p.In][p.ParamName]; !exist {
			dupCheck[p.In][p.ParamName] = "local"
			allParams = append(allParams, p)
		} else {
			return nil, fmt.Errorf("duplicate local parameter %s/%s", p.In, p.ParamName)
		}
	}
	for _, p := range globalParams {
		if dupCheck[p.In] == nil {
			dupCheck[p.In] = make(map[string]string)
		}
		if t, exist := dupCheck[p.In][p.ParamName]; !exist {
			dupCheck[p.In][p.ParamName] = "global"
			allParams = append(allParams, p)
		} else if t == "global" {
			return nil, fmt.Errorf("duplicate global parameter %s/%s", p.In, p.ParamName)
		}
	}

	return allParams, nil
}
