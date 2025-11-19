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
	"iter"
	"strconv"
	"strings"

	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// ResponseDefinition describes a response.
type ResponseDefinition struct {
	SuccessStatusCode int
	Success           *ResponseContentDefinition
	Error             *ResponseContentDefinition
	All               map[int]*ResponseContentDefinition
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
	StatusCode   int
	Headers      map[string]GoSchema
}

func getOperationResponses(operationID string, responses *v3high.Responses, options ParseOptions) (*ResponseDefinition, []TypeDefinition, error) {
	if responses == nil {
		return nil, nil, nil
	}

	var (
		successCode     int
		errorCode       int
		fstErrorCode    int
		fstSuccessCode  int
		typeDefinitions []TypeDefinition
	)

	all := make(map[int]*ResponseContentDefinition)
	defaultResponse := responses.Default

	// we just need success and error responses
	for statusCode, response := range responses.Codes.FromOldest() {
		if response == nil {
			continue
		}

		isSuccess := false
		refType := ""
		var err error

		headers, err := generateResponseHeadersSchema(response.Headers.FromOldest(), operationID, options)
		if err != nil {
			return nil, nil, err
		}

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
			errorCode = status
		} else {
			continue
		}

		// we need to set the error in response out of all error codes.
		// so we pick the first one.
		// TODO: consider having that in parse options.
		if fstErrorCode == 0 && !isSuccess {
			fstErrorCode = status
		}

		if fstSuccessCode == 0 && isSuccess {
			fstSuccessCode = status
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
				successDefinition := &ResponseContentDefinition{
					IsSuccess:    isSuccess,
					Description:  response.Description,
					ResponseName: "struct{}",
					StatusCode:   status,
					Headers:      headers,
				}
				all[status] = successDefinition
			}
			continue
		}

		ref := response.GoLow().GetReference()

		typeSuffix := "Response"
		if !isSuccess {
			typeSuffix = "ErrorResponse"
		}

		options = options.
			WithReference(ref).
			WithPath([]string{operationID, typeSuffix})
		contentSchema, err := GenerateGoSchema(content.Schema, options)
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
			contentSchema.DefineViaAlias = true
		}

		tag := ""
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

		codeName := strconv.Itoa(status)
		baseName := operationID + typeSuffix
		nameSuffixes := []string{tag, tag + codeName}
		responseName := generateTypeName(options.currentTypes, baseName, nameSuffixes)

		if contentSchema.ArrayType != nil {
			contentSchema, _ = replaceInlineTypes(contentSchema, options)
		}

		td := TypeDefinition{
			Name:           responseName,
			Schema:         contentSchema,
			SpecLocation:   SpecLocationResponse,
			NeedsMarshaler: needsMarshaler(contentSchema),
		}
		options.AddType(td)
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
			StatusCode:   status,
			Headers:      headers,
		}
		all[status] = rcd
	}

	if successCode == 0 {
		successCode = 204
		successDefinition := &ResponseContentDefinition{
			IsSuccess:    true,
			Description:  "No Content",
			ResponseName: "struct{}",
			StatusCode:   successCode,
		}

		all[successCode] = successDefinition
	}

	if errorCode == 0 && defaultResponse != nil {
		errorCode = 500
		fstErrorCode = 500
		typeSuffix := "ErrorResponse"
		content := defaultResponse.Content.First()

		ref := ""
		contentType := "application/json"
		var (
			contentSchema GoSchema
			err           error
			refType       string
			contentVal    *v3high.MediaType
		)

		if content != nil {
			contentType, contentVal = content.Key(), content.Value()
			ref = contentVal.Schema.GetReference()

			opts := options.WithReference(ref).WithPath([]string{operationID, typeSuffix})
			contentSchema, err = GenerateGoSchema(contentVal.Schema, opts)
			if err != nil {
				return nil, nil, fmt.Errorf("error generating request body definition: %w", err)
			}
		}

		if ref != "" {
			refType, err = refPathToGoType(ref)
			if err != nil {
				return nil, nil, fmt.Errorf("error turning reference (%s) into a Go type: %w", ref, err)
			}
		}

		if !contentSchema.IsZero() {
			if refType != "" {
				contentSchema.RefType = refType
			}
			responseName := operationID + typeSuffix
			if contentSchema.ArrayType != nil {
				contentSchema, _ = replaceInlineTypes(contentSchema, options)
			}
			td := TypeDefinition{
				Name:           responseName,
				Schema:         contentSchema,
				SpecLocation:   SpecLocationResponse,
				NeedsMarshaler: needsMarshaler(contentSchema),
			}
			options.AddType(td)
			typeDefinitions = append(typeDefinitions, td)
			typeDefinitions = append(typeDefinitions, contentSchema.AdditionalTypes...)

			errHeaders, err := generateResponseHeadersSchema(defaultResponse.Headers.FromOldest(), operationID, options)
			if err != nil {
				return nil, nil, fmt.Errorf("error generating response headers schema: %w", err)
			}

			errorDefinition := &ResponseContentDefinition{
				ResponseName: responseName,
				IsSuccess:    false,
				Description:  defaultResponse.Description,
				Schema:       contentSchema,
				Ref:          refType,
				ContentType:  contentType,
				StatusCode:   errorCode,
				Headers:      errHeaders,
			}
			all[errorCode] = errorDefinition
		}
	}

	res := &ResponseDefinition{
		SuccessStatusCode: successCode,
		Success:           all[successCode],
		Error:             all[fstErrorCode],
		All:               all,
	}

	return res, typeDefinitions, nil
}

func generateResponseHeadersSchema(headers iter.Seq2[string, *v3high.Header], operationID string, options ParseOptions) (map[string]GoSchema, error) {
	res := make(map[string]GoSchema)
	opts := options.WithReference("").WithPath([]string{operationID, "Header"})

	for hName, hdrs := range headers {
		hSchema, err := GenerateGoSchema(hdrs.Schema, opts)
		if err != nil {
			return nil, err
		}
		res[hName] = hSchema
	}
	return res, nil
}
