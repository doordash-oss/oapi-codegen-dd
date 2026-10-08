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
	"cmp"
	"fmt"
	"iter"
	"slices"
	"strconv"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
)

// ResponseDefinition describes a response.
type ResponseDefinition struct {
	SuccessStatusCode int
	Success           *ResponseContentDefinition
	Error             *ResponseContentDefinition
	All               map[int]*ResponseContentDefinition

	// Successes lists every 2xx response documented under an exact status
	// code, in ascending order. Drives `client-with-response` envelope
	// generation, which needs a deterministic per-status iteration.
	Successes []*ResponseContentDefinition

	// Errors lists every non-2xx response documented under an exact status
	// code, in ascending order. Same motivation as Successes - the envelope
	// populates `JSON404`/`JSON500` fields per documented status.
	Errors []*ResponseContentDefinition

	// StatusRanges lists the responses documented under a range such as
	// `4XX`, in ascending order. The envelope client matches them only after
	// every exact code, so an explicit `404` takes precedence over `4XX`.
	StatusRanges []*ResponseContentDefinition

	// Default is the `default` response, which the envelope client matches
	// last: it covers every status nothing more specific documents. Next to
	// explicit error codes it is only resolved when ParseOptions.ClientWithResponse
	// is set, because nothing else consumes it there.
	Default *ResponseContentDefinition

	// Streams lists every sequential media type documented on a success
	// response, in ascending status order. Populated independently of which
	// media type the primary method selected, so an operation offering both
	// `application/json` and `text/event-stream` appears here without
	// displacing the JSON response.
	Streams []*StreamResponseDefinition
}

// StreamResponseDefinition describes one sequential response. Kept separate from
// ResponseContentDefinition so the primary method keeps its own media type and
// Go types; only the streaming siblings read this.
type StreamResponseDefinition struct {
	StatusCode  int
	ContentType string
	// Framing is "sse" or "lines".
	Framing string
	// ItemName is the Go type of one frame, e.g. "Event", or "[]byte" when
	// there is no per-item schema that can be JSON-decoded.
	ItemName string
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

	// StatusCode is the documented status code. For a range or `default` it
	// is the stand-in code handler generation uses: 200 for `2XX`, 400 for
	// both `4XX` and `5XX`, 200 or 500 for `default`. The envelope client
	// matches StatusRange and IsDefault instead.
	StatusCode int

	// StatusRange is the class digit of a range such as `4XX` (4), or zero
	// when the response documents an exact status code.
	StatusRange int

	// IsDefault is true for the `default` response.
	IsDefault bool

	Headers map[string]GoSchema
	// IsRaw is true for unsupported content types (XML, form-urlencoded, etc.)
	// that require the user to handle marshaling manually.
	IsRaw bool

	// IsStream is true when the media type the primary method selected is
	// sequential. It does not change the primary method's shape; it lets
	// callers that cannot stream (the MCP tool template) refuse instead of
	// blocking forever.
	IsStream bool

	// componentResponse is true when the response's type comes from a
	// component response it references, which the envelope gave no typed
	// field before ranges and `default` got their own.
	componentResponse bool
}

// HasStream reports whether streaming siblings can be generated.
func (r ResponseDefinition) HasStream() bool {
	return len(r.Streams) > 0
}

// PrimaryStream returns the sequential response at the lowest success status,
// whose item type and media type the streaming sibling uses.
func (r ResponseDefinition) PrimaryStream() *StreamResponseDefinition {
	if len(r.Streams) == 0 {
		return nil
	}
	return r.Streams[0]
}

// StreamAt returns the sequential response documented at a status, or nil.
func (r ResponseDefinition) StreamAt(status int) *StreamResponseDefinition {
	for _, sd := range r.Streams {
		if sd.StatusCode == status {
			return sd
		}
	}
	return nil
}

// StreamFor returns the sequential response streamed in place of rcd's body,
// or nil. A range sharing its stand-in code with an exact status, such as
// `2XX` next to `200`, shares the stream too; only the response All holds at
// that code owns it, so the envelope declares the field once.
func (r ResponseDefinition) StreamFor(rcd *ResponseContentDefinition) *StreamResponseDefinition {
	if r.All[rcd.StatusCode] != rcd {
		return nil
	}
	return r.StreamAt(rcd.StatusCode)
}

// EnvelopeResponses returns every response the envelope client matches, in
// the order it tries them: exact codes, then ranges, then `default`.
func (r ResponseDefinition) EnvelopeResponses() []*ResponseContentDefinition {
	responses := slices.Concat(r.Successes, r.Errors, r.StatusRanges)
	if r.Default != nil {
		responses = append(responses, r.Default)
	}
	return responses
}

// LegacyBodyField returns the name the envelope gave a range's body field
// before ranges were matched by class, such as JSON400 for an inline `4XX`,
// so it stays as a deprecated alias of the current field. It is "" when there
// was none - only the range All holds at its stand-in code had one - or when
// an exact code documented there owns that name now.
func (r ResponseDefinition) LegacyBodyField(rcd *ResponseContentDefinition) string {
	// A response referencing a component response had no typed field.
	if rcd.StatusRange == 0 || rcd.componentResponse || !rcd.hasBodyField() || r.All[rcd.StatusCode] != rcd {
		return ""
	}
	name := rcd.NameTag + strconv.Itoa(rcd.StatusCode)
	for _, exact := range slices.Concat(r.Successes, r.Errors) {
		if exact.hasBodyField() && exact.NameTag+strconv.Itoa(exact.StatusCode) == name {
			return ""
		}
	}
	return name
}

// HasLegacyHeaders reports whether a range or `default` keeps the headers
// field and type the envelope declared under its stand-in code, such as
// Headers500 for `default`, as deprecated aliases. As with LegacyBodyField,
// only the response All holds at that code had them, and an exact code there
// documenting headers owns the names now.
func (r ResponseDefinition) HasLegacyHeaders(rcd *ResponseContentDefinition) bool {
	if rcd.isExactStatus() || len(rcd.Headers) == 0 || r.All[rcd.StatusCode] != rcd {
		return false
	}
	for _, exact := range slices.Concat(r.Successes, r.Errors) {
		if exact.StatusCode == rcd.StatusCode && len(exact.Headers) > 0 {
			return false
		}
	}
	return true
}

// StatusName names the response in the envelope client's field and type
// names: the status code ("404"), the range ("4XX"), or "Default".
func (r ResponseContentDefinition) StatusName() string {
	if r.IsDefault {
		return "Default"
	}
	return statusName(r.StatusCode, r.StatusRange)
}

// isExactStatus reports whether the response documents a single status code
// rather than a range or `default`.
func (r ResponseContentDefinition) isExactStatus() bool {
	return r.StatusRange == 0 && !r.IsDefault
}

// hasBodyField reports whether the envelope declares a typed body field for
// the response.
func (r ResponseContentDefinition) hasBodyField() bool {
	return r.ResponseName != "struct{}" && !r.IsRaw && r.NameTag != ""
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

	// Every response the envelope client can match. Unlike `all`, which puts a
	// range or `default` at a stand-in code, it keeps each response as
	// documented, so `4XX` and `5XX` do not overwrite each other and an exact
	// code can take precedence over the range covering it.
	var documented []*ResponseContentDefinition

	// Content of every documented success status, plus any schema the primary
	// pass already generated for a sequential media type. Resolved into
	// ResponseDefinition.Streams after the loop, so detection never depends on
	// which media type the primary pass selected.
	streamContents := make(map[int]*orderedmap.Map[string, *v3high.MediaType])
	streamFallbacks := make(map[int]streamFallback)

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

		res := &ResponseDefinition{
			SuccessStatusCode: successCode,
			Success:           successDefinition,
			Error:             nil,
			All:               all,
		}
		res.setEnvelopeResponses([]*ResponseContentDefinition{successDefinition})
		return res, nil, nil
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
		statusRange := statusRangeOf(statusCode)

		if status >= 200 && status < 300 {
			isSuccess = true
			successCode = status
		} else if status >= 300 && status < 600 {
			isSuccess = false
			errorCode = status
		} else {
			continue
		}

		if isSuccess && response.Content != nil {
			streamContents[status] = response.Content
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

		// A sequential media type may carry only `itemSchema` (OpenAPI 3.2) and
		// leave `schema` unset. Fall back to it so the response still counts as
		// having content; the whole-body type is []byte either way.
		bodySchema := responseBodySchema(content, contentType)

		if content == nil || bodySchema == nil {
			bodyless := &ResponseContentDefinition{
				IsSuccess:    isSuccess,
				Description:  response.Description,
				ResponseName: "struct{}",
				StatusCode:   status,
				StatusRange:  statusRange,
				Headers:      headers,
				// Preserve the declared media type even when the schema
				// is empty (e.g. `text/html: {}`); strict response
				// validators rely on it.
				ContentType: contentType,
			}
			// An error without a body has nothing to decode, so only the
			// envelope client sees it, to keep its precedence over a range
			// or `default`.
			if isSuccess {
				all[status] = bodyless
			}
			documented = append(documented, bodyless)
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
		contentSchema, err := GenerateGoSchema(bodySchema, options)
		if err != nil {
			return nil, nil, fmt.Errorf("error generating request body definition: %w", err)
		}
		if contentSchema.IsZero() {
			continue
		}

		// When the selected media type is itself sequential, remember the schema
		// so the stream pass reuses it instead of walking it twice.
		if _, isStream := streamFraming(contentType); isStream {
			streamFallbacks[status] = streamFallback{proxy: bodySchema, schema: contentSchema}
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
		tag := responseNameTag(contentType)

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
			// A range's stand-in code would name a 5XX type after 400.
			codeName := statusName(status, statusRange)
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
		_, isStream := streamFraming(contentType)

		rcd := &ResponseContentDefinition{
			ResponseName: responseName,
			IsSuccess:    isSuccess,
			Description:  response.Description,
			Schema:       contentSchema,
			Ref:          refType,
			ContentType:  contentType,
			NameTag:      tag,
			StatusCode:   status,
			StatusRange:  statusRange,
			Headers:      headers,
			IsRaw:        isRaw,
			IsStream:     isStream,

			componentResponse: componentTypeExists,
		}
		all[status] = rcd
		documented = append(documented, rcd)
	}

	// When no explicit success is documented but `default` has content, treat
	// `default` as the success at 200. OpenAPI validators interpret `default`
	// as covering any unlisted status code (including 2xx), so the spec's
	// schema is what they check against. Fabricating a 204 struct{} here
	// would make mocks return content that doesn't satisfy the real schema.
	defaultAsSuccess := false
	if successCode == 0 && defaultResponse != nil && defaultResponse.Content != nil && defaultResponse.Content.First() != nil {
		rcd, tds, err := buildDefaultResponseDefinition(operationID, defaultResponse, 200, true, false, options)
		if err != nil {
			return nil, nil, err
		}
		if rcd != nil {
			successCode = 200
			all[200] = rcd
			documented = append(documented, rcd)
			typeDefinitions = append(typeDefinitions, tds...)
			defaultAsSuccess = true
			// A `default` standing in as the success can be sequential too, so
			// it has to reach the stream pass like any explicit status.
			streamContents[200] = defaultResponse.Content
		}
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
		documented = append(documented, successDefinition)
	}

	// Handler generation answers errors with the first explicit error code, so
	// it only needs `default` when none is documented. The envelope client
	// matches `default` against every status nothing else documents, so it
	// needs it next to explicit error codes too.
	alongsideErrors := errorCode != 0
	if defaultResponse != nil && !defaultAsSuccess && (!alongsideErrors || options.ClientWithResponse) {
		rcd, tds, err := buildDefaultResponseDefinition(operationID, defaultResponse, 500, false, alongsideErrors, options)
		if err != nil {
			return nil, nil, err
		}
		if rcd != nil {
			if !alongsideErrors {
				fstErrorCode = 500
				all[500] = rcd
			}
			documented = append(documented, rcd)
			typeDefinitions = append(typeDefinitions, tds...)
		}
	}

	streams, streamTypes, err := collectStreamResponses(operationID, streamContents, streamFallbacks, options)
	if err != nil {
		return nil, nil, err
	}
	typeDefinitions = append(typeDefinitions, streamTypes...)

	res := &ResponseDefinition{
		SuccessStatusCode: successCode,
		Success:           all[successCode],
		Error:             all[fstErrorCode],
		All:               all,
		Streams:           streams,
	}
	res.setEnvelopeResponses(documented)

	return res, typeDefinitions, nil
}

// buildDefaultResponseDefinition turns a `default` response into a
// ResponseContentDefinition installed at the given status. Used both when no
// explicit success is documented (install as 200 success) and when no
// explicit error is documented (install as 500 error).
//
// With alongsideErrors it is built for the envelope client next to explicit
// error codes. The first of those already owns the plain type name and schema
// path, so `default` gets its own to keep nested types apart.
func buildDefaultResponseDefinition(operationID string, defaultResponse *v3high.Response, status int, isSuccess, alongsideErrors bool, options ParseOptions) (*ResponseContentDefinition, []TypeDefinition, error) {
	typeSuffix := "ErrorResponse"
	if isSuccess {
		typeSuffix = "Response"
	}

	pathParts := []string{operationID, typeSuffix}
	if alongsideErrors {
		pathParts = append(pathParts, "default")
	}

	content := defaultResponse.Content.First()
	ref := ""
	contentType := "application/json"
	var (
		contentSchema GoSchema
		err           error
		refType       string
		contentVal    *v3high.MediaType
	)

	var bodySchema *base.SchemaProxy
	if content != nil {
		contentType, contentVal = content.Key(), content.Value()
		bodySchema = responseBodySchema(contentVal, contentType)
		if bodySchema != nil {
			ref = bodySchema.GetReference()

			opts := options.WithReference(ref).WithPath(pathParts)
			contentSchema, err = GenerateGoSchema(bodySchema, opts)
			if err != nil {
				return nil, nil, fmt.Errorf("error generating request body definition: %w", err)
			}
		}
	}

	// Only a component schema has a type of its own to point at. A JSON pointer
	// into another operation's response, as bundled specs use, names nothing
	// generated: GenerateGoSchema already produced the type inline.
	if ref != "" && isStandardComponentReference(ref) {
		refType, err = refPathToGoType(ref)
		if err != nil {
			return nil, nil, fmt.Errorf("error turning reference (%s) into a Go type: %w", ref, err)
		}
	}

	if contentSchema.IsZero() {
		return nil, nil, nil
	}

	var typeDefinitions []TypeDefinition
	_, isStream := streamFraming(contentType)

	// Coerce raw content types (XML, CSV, */*, etc.) to []byte. Mirrors the
	// per-status path; without it, a `default` response with a non-JSON
	// content type would emit a typed Body that the handler template's
	// "unknown content type" branch then tries to w.Write as []byte.
	isRaw := isRawContentType(contentType)
	if isRaw {
		contentSchema = GoSchema{
			GoType:         "[]byte",
			DefineViaAlias: true,
			Description:    contentSchema.Description,
		}
		refType = ""
	}

	if refType != "" {
		contentSchema.RefType = refType
	}
	tag := responseNameTag(contentType)
	responseName := operationID + typeSuffix
	if alongsideErrors {
		responseName = options.typeTracker.generateUniqueNameWithSuffixes(responseName, []string{tag, tag + "Default"})
	}
	if contentSchema.ArrayType != nil {
		contentSchema, _ = replaceInlineTypes(contentSchema, options)
	}
	// When the default schema $refs a component (e.g. an error/event struct)
	// AND the content type forces us to []byte, the operation-level alias
	// name can collide with the component type. The component is already
	// registered as a struct; registering a []byte alias under the same
	// name silently drops it and the client template still ends up with
	// `result := SomeStruct(bodyBytes)`. Disambiguate the alias name so
	// both types co-exist.
	if isRaw && options.typeTracker.Exists(responseName) {
		responseName = options.typeTracker.generateUniqueName(responseName)
	}
	td := TypeDefinition{
		Name:           responseName,
		Schema:         contentSchema,
		SpecLocation:   SpecLocationResponse,
		NeedsMarshaler: needsMarshaler(contentSchema),
	}
	options.typeTracker.register(td, "")
	typeDefinitions = append(typeDefinitions, td)

	for _, additionalType := range contentSchema.AdditionalTypes {
		if _, exists := options.typeTracker.LookupByName(additionalType.Name); !exists {
			typeDefinitions = append(typeDefinitions, additionalType)
			options.typeTracker.register(additionalType, "")
		}
	}

	headers, err := generateResponseHeadersSchema(defaultResponse.Headers.FromOldest(), operationID, options)
	if err != nil {
		return nil, nil, fmt.Errorf("error generating response headers schema: %w", err)
	}

	rcd := &ResponseContentDefinition{
		ResponseName: responseName,
		IsSuccess:    isSuccess,
		Description:  defaultResponse.Description,
		Schema:       contentSchema,
		Ref:          refType,
		ContentType:  contentType,
		NameTag:      tag,
		StatusCode:   status,
		IsDefault:    true,
		Headers:      headers,
		IsRaw:        isRaw,
		IsStream:     isStream,
	}
	return rcd, typeDefinitions, nil
}

// setEnvelopeResponses sorts every documented response into the lists the
// envelope client matches, each in ascending order: Successes and Errors for
// exact codes, then StatusRanges, then Default.
//
// An error without a body has nothing to decode, so it is left to the
// unexpected-status fallback - unless a range or `default` would claim its
// status instead. Then it keeps an entry of its own, since OpenAPI gives the
// more specific response precedence.
func (r *ResponseDefinition) setEnvelopeResponses(documented []*ResponseContentDefinition) {
	var bodyless []*ResponseContentDefinition
	for _, rcd := range documented {
		switch {
		case rcd.IsDefault:
			r.Default = rcd
		case !rcd.IsSuccess && rcd.ResponseName == "struct{}":
			bodyless = append(bodyless, rcd)
		case rcd.StatusRange != 0:
			r.StatusRanges = append(r.StatusRanges, rcd)
		case rcd.IsSuccess:
			r.Successes = append(r.Successes, rcd)
		default:
			r.Errors = append(r.Errors, rcd)
		}
	}

	for _, rcd := range bodyless {
		if !r.coveredByFallback(rcd) {
			continue
		}
		if rcd.StatusRange != 0 {
			r.StatusRanges = append(r.StatusRanges, rcd)
		} else {
			r.Errors = append(r.Errors, rcd)
		}
	}

	byStatus := func(a, b *ResponseContentDefinition) int { return cmp.Compare(a.StatusCode, b.StatusCode) }
	slices.SortFunc(r.Successes, byStatus)
	slices.SortFunc(r.Errors, byStatus)
	slices.SortFunc(r.StatusRanges, func(a, b *ResponseContentDefinition) int {
		return cmp.Compare(a.StatusRange, b.StatusRange)
	})
}

// coveredByFallback reports whether a range or `default` would match the
// statuses rcd documents, were rcd not matched before them.
func (r *ResponseDefinition) coveredByFallback(rcd *ResponseContentDefinition) bool {
	if r.Default != nil {
		return true
	}
	if rcd.StatusRange != 0 {
		return false
	}
	for _, rng := range r.StatusRanges {
		if rng.StatusRange == rcd.StatusCode/100 {
			return true
		}
	}
	return false
}

// statusName names an exact status code ("404"), or the range ("4XX") when
// statusRange is set - never the range's stand-in code.
func statusName(code, statusRange int) string {
	if statusRange != 0 {
		return strconv.Itoa(statusRange) + "XX"
	}
	return strconv.Itoa(code)
}

// statusRangeOf returns the class digit of a response key documenting a
// status range, such as 4 for "4XX", or zero for any other key.
func statusRangeOf(key string) int {
	if len(key) != 3 || !strings.EqualFold(key[1:], "XX") || key[0] < '1' || key[0] > '5' {
		return 0
	}
	return int(key[0] - '0')
}

// responseNameTag returns the tag a response's media type contributes to its
// Go type and envelope field names, such as "JSON" in JSON404, or "" when
// the media type has none.
func responseNameTag(contentType string) string {
	switch {
	case contentType == "application/json":
		return "JSON"
	case isMediaTypeJson(contentType):
		return mediaTypeToCamelCase(contentType)
	case contentType == "application/x-www-form-urlencoded":
		return "Formdata"
	case strings.HasPrefix(contentType, "multipart/"):
		return "Multipart"
	case contentType == "text/plain":
		return "Text"
	case contentType == "text/html":
		return "HTML"
	}
	return ""
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

// responseBodySchema returns the schema describing a response body. For a
// sequential media type, `itemSchema` stands in for a missing `schema` so the
// response is not mistaken for having no content.
func responseBodySchema(content *v3high.MediaType, contentType string) *base.SchemaProxy {
	if content == nil {
		return nil
	}
	if content.Schema != nil {
		return content.Schema
	}
	if _, isStream := streamFraming(contentType); isStream {
		return content.ItemSchema
	}
	return nil
}

// isRawContentType returns true for content types that require manual marshaling
// (XML, YAML, ndjson, etc.) and should use []byte as the response type.
//
// Note: only `application/json` and `*+json` are JSON-encodable as a single
// object. ndjson/jsonl variants are recognized by `isMediaTypeJson` for
// naming/tagging purposes, but they need raw byte framing on the wire, so
// they fall through to []byte here.
func isRawContentType(contentType string) bool {
	return contentType != "" &&
		contentType != "application/json" &&
		!strings.HasPrefix(contentType, "application/json;") &&
		!strings.HasSuffix(contentType, "+json") &&
		!strings.Contains(contentType, "+json;") &&
		contentType != "text/plain" &&
		!strings.HasPrefix(contentType, "text/plain;") &&
		contentType != "text/html" &&
		!strings.HasPrefix(contentType, "text/html;") &&
		contentType != "application/octet-stream" &&
		!strings.HasPrefix(contentType, "application/octet-stream;") &&
		contentType != "application/x-www-form-urlencoded" &&
		!strings.HasPrefix(contentType, "application/x-www-form-urlencoded;")
}
