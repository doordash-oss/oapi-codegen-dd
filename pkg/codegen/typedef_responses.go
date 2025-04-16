package codegen

import (
	"fmt"
	"strconv"
	"strings"

	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// ResponseDefinition describes a response.
type ResponseDefinition struct {
	SuccessStatusCode int
	Success           *ResponseContentDefinition
	Error             *ResponseContentDefinition
}

// ResponseContentDefinition describes Operation response.
// GoSchema is the schema describing this content.
// ContentType is the content type corresponding to the body, eg, application/json.
// NameTag is the tag for the type name, such as JSON, in which case we will produce "Response200JSONContent".
// ResponseName is the name of the response.
// Description is the description of the response.
// Ref is the reference to the response.
// IsSuccess is true if the response is a success response.
type ResponseContentDefinition struct {
	Schema      GoSchema
	ContentType string
	NameTag     string

	ResponseName string
	Description  string
	Ref          string
	IsSuccess    bool
}

func getOperationResponses(operationID string, responses *v3high.Responses, options ParseOptions) (*ResponseDefinition, []TypeDefinition, error) {
	var (
		successDefinition *ResponseContentDefinition
		errorDefinition   *ResponseContentDefinition
		successCode       int
	)

	var typeDefinitions []TypeDefinition

	// we just need success and error responses
	for statusCode, response := range responses.Codes.FromOldest() {
		if response == nil {
			continue
		}
		isSuccess := false
		refType := ""
		var err error

		status, err := strconv.Atoi(statusCode)
		if err != nil {
			if statusCode == "default" || strings.ToLower(statusCode) == "2xx" {
				status = 200
			} else if strings.ToLower(statusCode) == "4xx" || strings.ToLower(statusCode) == "5xx" {
				status = 400
			} else {
				return nil, nil, fmt.Errorf("error parsing status code %s: %w", statusCode, err)
			}
		}

		if status >= 200 && status < 300 {
			isSuccess = true
			successCode = status
		} else if status >= 300 && status < 600 {
			isSuccess = false
		} else {
			continue
		}

		// pick one schema
		// TODO: prefer json content type
		if isSuccess && successDefinition != nil {
			continue
		}
		if !isSuccess && errorDefinition != nil {
			continue
		}

		typeSuffix := "Response"
		if !isSuccess {
			typeSuffix = "ErrorResponse"
		}

		var (
			contentType string
			content     *v3high.MediaType
		)

		if response.Content != nil {
			if pair, ok := response.Content.Get("application/json"); ok {
				contentType, content = "application/json", pair
			} else {
				if v := response.Content.First(); v != nil {
					contentType, content = v.Key(), v.Value()
				}
			}
		}

		if content == nil {
			if isSuccess {
				successDefinition = &ResponseContentDefinition{
					IsSuccess:    isSuccess,
					Description:  response.Description,
					ResponseName: "struct{}",
				}
			}
			continue
		}

		ref := response.GoLow().GetReference()

		contentSchema, err := GenerateGoSchema(content.Schema, ref, []string{operationID, typeSuffix}, options)
		if err != nil {
			return nil, nil, fmt.Errorf("error generating request body definition: %w", err)
		}
		if contentSchema.IsZero() {
			continue
		}

		if ref != "" {
			// Convert the reference path to Go type
			refType, err = refPathToGoType(ref)
			if err != nil {
				return nil, nil, fmt.Errorf("error turning reference (%s) into a Go type: %w", ref, err)
			}
			contentSchema.RefType = refType
		}

		tag := ""
		// TODO: check if needed
		switch {
		case contentType == "application/json":
			tag = "JSON"
		case isMediaTypeJson(contentType):
			tag = mediaTypeToCamelCase(contentType)
		case contentType == "application/x-www-form-urlencoded":
			tag = "Formdata"
		case strings.HasPrefix(contentType, "multipart/"):
			tag = "Multipart"
		case contentType == "text/plain":
			tag = "Text"
		}

		responseName := operationID + typeSuffix
		td := TypeDefinition{
			Name:         responseName,
			Schema:       contentSchema,
			SpecLocation: SpecLocationResponse,
		}
		typeDefinitions = append(typeDefinitions, td)
		typeDefinitions = append(typeDefinitions, contentSchema.AdditionalTypes...)

		rcd := &ResponseContentDefinition{
			ResponseName: responseName,
			IsSuccess:    isSuccess,
			Description:  response.Description,
			Schema:       contentSchema,
			Ref:          refType,
			ContentType:  contentType,
			NameTag:      tag,
		}

		if isSuccess {
			successDefinition = rcd
		} else {
			errorDefinition = rcd
		}
	}

	if successDefinition == nil {
		successDefinition = &ResponseContentDefinition{
			IsSuccess:    true,
			Description:  "No Content",
			ResponseName: "struct{}",
		}
		successCode = 204
	}

	res := &ResponseDefinition{
		SuccessStatusCode: successCode,
		Success:           successDefinition,
		Error:             errorDefinition,
	}

	return res, typeDefinitions, nil
}
