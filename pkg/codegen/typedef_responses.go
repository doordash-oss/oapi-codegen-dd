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
	// IsRaw is true for unsupported content types (XML, form-urlencoded, etc.)
	// that require the user to handle marshaling manually.
	IsRaw bool
}

func getOperationResponses(operationID string, responses *v3high.Responses, options ParseOptions) (*ResponseDefinition, []TypeDefinition, error) {
	var (
		successCode          int
		errorCode            int
		fstErrorCode         int
		fstSuccessCode       int
		typeDefinitions      []TypeDefinition
		errorAliasRegistered bool // Track if we've already registered the error response alias
	)

	all := make(map[int]*ResponseContentDefinition)

	// If responses is nil, create a default 204 No Content response
	if responses == nil {
		successCode = 204
		successDefinition := &ResponseContentDefinition{
			IsSuccess:    true,
			Description:  "No Content",
			ResponseName: "struct{}",
			StatusCode:   successCode,
		}
		all[successCode] = successDefinition

		return &ResponseDefinition{
			SuccessStatusCode: successCode,
			Success:           successDefinition,
			Error:             nil,
			All:               all,
		}, nil, nil
	}

	defaultResponse := responses.Default

	// we just need success and error responses
	for statusCode, response := range responses.Codes.FromOldest() {
		if response == nil {
			continue
		}

		isSuccess := false
		refType := ""
		isComponentRef := false
		var err error

		// Check if this response is a $ref to a component response
		responseRef := response.GoLow().GetReference()
		if responseRef != "" {
			refType, err = refPathToGoType(responseRef)
			if err != nil {
				return nil, nil, fmt.Errorf("error turning reference (%s) into a Go type: %w", responseRef, err)
			}
			// Check if this is a component reference (vs a path reference)
			isComponentRef = strings.HasPrefix(responseRef, "#/components/")
		}

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

		if content == nil || content.Schema == nil {
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

		typeSuffix := "Response"
		if !isSuccess {
			typeSuffix = "ErrorResponse"
		}

		// Don't pass reference for responses - we want actual types, not aliases
		// This allows Error() methods to be generated on error response types
		// Also set SpecLocationResponse so that writeOnly fields are not marked as required
		// Include status code in path only for non-first responses to disambiguate
		// nested types (like array items) when multiple responses have the same structure
		pathParts := []string{operationID, typeSuffix}
		isFirstOfKind := (isSuccess && status == fstSuccessCode) || (!isSuccess && status == fstErrorCode)
		if !isFirstOfKind {
			pathParts = append(pathParts, statusCode)
		}
		options = options.
			WithReference("").
			WithPath(pathParts).
			WithSpecLocation(SpecLocationResponse)
		contentSchema, err := GenerateGoSchema(content.Schema, options)
		if err != nil {
			return nil, nil, fmt.Errorf("error generating request body definition: %w", err)
		}
		if contentSchema.IsZero() {
			continue
		}

		// For raw content types (XML, YAML, etc.), override the schema to []byte
		// since we can't automatically unmarshal these formats.
		if isRawContentType(contentType) {
			contentSchema = GoSchema{
				GoType:         "[]byte",
				DefineViaAlias: true,
				Description:    contentSchema.Description,
			}
		}

		var responseName string
		tag := ""

		// If this is a component reference AND the type exists (was processed by getComponentResponses),
		// use the component name and don't create a duplicate TypeDefinition.
		// Otherwise, generate a dynamic name and create a TypeDefinition.
		componentTypeName := ""
		if isComponentRef {
			componentTypeName = schemaNameToTypeName(refType)
		}

		// Check if the component type actually exists AND is a response type.
		// This is important because a component response might have the same name as a schema type.
		// For example, components/responses/BusinessGroup might be an array of components/schemas/BusinessGroup.
		// In this case, the response type will be named BusinessGroupResponse (with Response suffix).
		componentTd, componentTypeExists := options.typeTracker.LookupByName(componentTypeName)
		componentTypeExists = componentTypeExists && componentTypeName != "" && componentTd.SpecLocation == SpecLocationResponse

		// If the component type doesn't exist with the original name, try with "Response" suffix.
		// This handles the case where the response type was renamed to avoid conflict with a schema type.
		if !componentTypeExists && componentTypeName != "" {
			componentTypeNameWithSuffix := componentTypeName + "Response"
			if td, exists := options.typeTracker.LookupByName(componentTypeNameWithSuffix); exists && td.SpecLocation == SpecLocationResponse {
				componentTypeName = componentTypeNameWithSuffix
				componentTd = td
				componentTypeExists = true
			}
		}

		if componentTypeExists {
			// For error responses, only create the alias for the first error response
			// to avoid overwriting the alias in the tracker with subsequent error responses.
			// The first error response is the one that will be used as the Error in ResponseDefinition.
			if isSuccess || !errorAliasRegistered {
				// Create an operation-specific alias to the component response/schema type
				// e.g., GetFilesErrorResponse = InvalidRequestError or GetFilesErrorResponse = ServiceError
				aliasName := operationID + typeSuffix

				// Check if error mapping is configured for this response type (the alias name).
				// If so, we cannot use an alias because aliases don't support methods,
				// and we need to generate an Error() method for error-mapped types.
				// Note: If error-mapping is configured for the component type (not the alias),
				// we keep the alias and let collectResponseErrors follow it to the component type.
				hasErrorMapping := len(options.ErrorMapping) > 0 && options.ErrorMapping[aliasName] != ""

				if hasErrorMapping {
					// Error mapping is configured - generate a full struct instead of alias
					// so we can attach the Error() method
					responseName = aliasName
					// Don't set componentTypeExists to false - we still want to use the component schema
					// but we need to generate a new type definition with the full schema
					td := TypeDefinition{
						Name:           aliasName,
						Schema:         componentTd.Schema,
						SpecLocation:   SpecLocationResponse,
						NeedsMarshaler: needsMarshaler(componentTd.Schema),
					}
					options.typeTracker.register(td, "")
					typeDefinitions = append(typeDefinitions, td)
				} else if existingTd, exists := options.typeTracker.LookupByName(aliasName); exists {
					// Check if the alias already exists (e.g., from a component response with the same name)
					// If so, check if it's the same type - if yes, reuse it; if no, generate a unique name
					if existingTd.Schema.RefType == componentTypeName {
						// Same type, reuse the existing alias
						responseName = aliasName
					} else {
						// Different type, generate a unique name
						aliasName = options.typeTracker.generateUniqueName(aliasName)
						td := TypeDefinition{
							Name:           aliasName,
							Schema:         GoSchema{RefType: componentTypeName, DefineViaAlias: true},
							SpecLocation:   SpecLocationResponse,
							NeedsMarshaler: false,
						}
						options.typeTracker.register(td, "")
						typeDefinitions = append(typeDefinitions, td)
						responseName = aliasName
					}
				} else {
					// Create a type alias
					td := TypeDefinition{
						Name:           aliasName,
						Schema:         GoSchema{RefType: componentTypeName, DefineViaAlias: true},
						SpecLocation:   SpecLocationResponse,
						NeedsMarshaler: false,
					}
					options.typeTracker.register(td, "")
					typeDefinitions = append(typeDefinitions, td)
					responseName = aliasName
				}

				if !isSuccess {
					errorAliasRegistered = true
				}
			} else {
				// For subsequent error responses, use the component type name directly
				responseName = componentTypeName
			}

			// Use the component's schema instead of the regenerated contentSchema.
			// This ensures we use the correct AdditionalTypes from the component
			// rather than duplicates generated with the response path context.
			contentSchema = componentTd.Schema
		} else {
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
			case contentType == "text/html":
				tag = "HTML"
			}

			codeName := strconv.Itoa(status)
			baseName := operationID + typeSuffix
			nameSuffixes := []string{tag, tag + codeName}
			responseName = options.typeTracker.generateUniqueNameWithSuffixes(baseName, nameSuffixes)

			if contentSchema.ArrayType != nil {
				contentSchema, _ = replaceInlineTypes(contentSchema, options)
			}

			// For error responses with alias types, we need to handle two cases:
			// 1. Primitive aliases (string, int, etc.): Convert to proper types so Error() can be added
			// 2. Component schema aliases (ServiceError, etc.): Keep as aliases, let collectResponseErrors
			//    follow them to the component schema which will get the Error() method
			if !isSuccess && contentSchema.DefineViaAlias {
				// Check if this is an alias to a registered type (component schema)
				if originalTd, exists := options.typeTracker.LookupByName(contentSchema.GoType); exists {
					// This is an alias to a component schema.
					// Only convert to full struct if error-mapping is configured for THIS response type.
					// Otherwise, keep the alias and let collectResponseErrors follow it.
					hasErrorMapping := len(options.ErrorMapping) > 0 && options.ErrorMapping[responseName] != ""
					if hasErrorMapping {
						// Copy the original schema but clear DefineViaAlias
						contentSchema = originalTd.Schema
						contentSchema.DefineViaAlias = false
					}
					// else: keep the alias, collectResponseErrors will follow it
				} else {
					// This is an alias to a primitive type (string, int, etc.)
					// Convert to a proper type so Error() method can be added
					contentSchema.DefineViaAlias = false
					contentSchema.IsPrimitiveAlias = true
				}
			}

			td := TypeDefinition{
				Name:           responseName,
				Schema:         contentSchema,
				SpecLocation:   SpecLocationResponse,
				NeedsMarshaler: needsMarshaler(contentSchema),
			}
			options.typeTracker.register(td, "")
			typeDefinitions = append(typeDefinitions, td)
			// Filter out AdditionalTypes that already exist in the type tracker
			// to avoid duplicating types that were already generated (e.g., from component schemas)
			for _, additionalType := range contentSchema.AdditionalTypes {
				if _, exists := options.typeTracker.LookupByName(additionalType.Name); !exists {
					typeDefinitions = append(typeDefinitions, additionalType)
					options.typeTracker.register(additionalType, "")
				}
			}
		}

		// IsRaw is true for unsupported content types that require manual marshaling
		// Use HasPrefix to handle content types with parameters (e.g., "text/html; charset=UTF-8")
		isRaw := isRawContentType(contentType)

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
			IsRaw:        isRaw,
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
			if contentVal.Schema != nil {
				ref = contentVal.Schema.GetReference()

				opts := options.WithReference(ref).WithPath([]string{operationID, typeSuffix})
				contentSchema, err = GenerateGoSchema(contentVal.Schema, opts)
				if err != nil {
					return nil, nil, fmt.Errorf("error generating request body definition: %w", err)
				}
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
			options.typeTracker.register(td, "")
			typeDefinitions = append(typeDefinitions, td)

			// Filter out AdditionalTypes that already exist in the type tracker
			for _, additionalType := range contentSchema.AdditionalTypes {
				if _, exists := options.typeTracker.LookupByName(additionalType.Name); !exists {
					typeDefinitions = append(typeDefinitions, additionalType)
					options.typeTracker.register(additionalType, "")
				}
			}

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

// isRawContentType returns true for content types that require manual marshaling
// (XML, YAML, etc.) and should use []byte as the response type.
func isRawContentType(contentType string) bool {
	return contentType != "" &&
		contentType != "application/json" &&
		!strings.HasPrefix(contentType, "application/json;") &&
		!isMediaTypeJson(contentType) &&
		contentType != "text/plain" &&
		!strings.HasPrefix(contentType, "text/plain;") &&
		contentType != "text/html" &&
		!strings.HasPrefix(contentType, "text/html;") &&
		contentType != "application/octet-stream" &&
		!strings.HasPrefix(contentType, "application/octet-stream;") &&
		contentType != "application/x-www-form-urlencoded" &&
		!strings.HasPrefix(contentType, "application/x-www-form-urlencoded;")
}
