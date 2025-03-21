package codegen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/doordash/oapi-codegen/v2/pkg/util"
	"github.com/getkin/kin-openapi/openapi3"
)

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
