// Copyright 2026 DoorDash, Inc.
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
	"log/slog"
	"mime"
	"sort"
	"strconv"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
)

const (
	streamFramingSSE   = "sse"
	streamFramingLines = "lines"

	// streamRawItemType is the frame type used when the spec gives no schema
	// that can be JSON-decoded, so the caller gets the bytes verbatim.
	streamRawItemType = "[]byte"
)

// streamFallback carries a schema the primary pass already generated. Walking
// the same schema twice would register its nested types under two names.
type streamFallback struct {
	proxy  *base.SchemaProxy
	schema GoSchema
}

// streamFraming returns the wire framing of a sequential response media type.
// runtime.IsSequentialMediaType answers the same question at request time from
// the response Content-Type.
func streamFraming(contentType string) (string, bool) {
	if !runtime.IsSequentialMediaType(contentType) {
		return "", false
	}
	if isEventStreamMediaType(contentType) {
		return streamFramingSSE, true
	}
	return streamFramingLines, true
}

// isEventStreamMediaType ignores any media type parameters.
func isEventStreamMediaType(contentType string) bool {
	if parsed, _, err := mime.ParseMediaType(contentType); err == nil {
		return parsed == "text/event-stream"
	}
	mediaType, _, _ := strings.Cut(contentType, ";")
	return strings.EqualFold(strings.TrimSpace(mediaType), "text/event-stream")
}

// warnUnconsumableStreams reports operations whose generated method will block
// because their only success media type is sequential, and points at the flag
// that fixes it. It scans media type names only, so it works when the detection
// pass is switched off - which is exactly when the warning matters.
func warnUnconsumableStreams(model *v3high.Document, cfg Configuration) {
	if cfg.Generate == nil || cfg.Generate.ClientStreaming {
		return
	}
	// Only client generation produces a method that can block.
	if !cfg.Generate.Client && !cfg.Generate.ClientWithResponse {
		return
	}
	if model == nil || model.Paths == nil || model.Paths.PathItems == nil {
		return
	}

	var blocking, streamable []string

	for path, item := range model.Paths.PathItems.FromOldest() {
		if item == nil {
			continue
		}
		for method, op := range item.GetOperations().FromOldest() {
			if op == nil || op.Responses == nil {
				continue
			}
			label := fmt.Sprintf("%s %s", strings.ToUpper(method), path)
			for _, response := range successResponses(op.Responses) {
				if response == nil || response.Content == nil {
					continue
				}
				sequential, other := classifyMediaTypes(response.Content)
				if !sequential {
					continue
				}
				// A non-sequential alternative is what the method selects, so
				// it returns a typed body and never blocks.
				if other {
					streamable = append(streamable, label)
				} else {
					blocking = append(blocking, label)
				}
				break
			}
		}
	}

	if len(blocking) > 0 {
		slog.Warn("generated client methods for these operations will block: their only documented success media type is sequential (SSE or line-delimited JSON), which cannot be buffered. Set 'client-streaming: true' under 'generate' to also emit a <Op>Stream method that consumes them",
			"operations", strings.Join(blocking, ", "))
	}
	if len(streamable) > 0 {
		slog.Info("these operations document a sequential success media type alongside a buffered one. Set 'client-streaming: true' under 'generate' to also emit a <Op>Stream method for them",
			"operations", strings.Join(streamable, ", "))
	}
}

// successResponses returns an operation's 2xx responses, plus `default` when no
// explicit success is documented, mirroring how the primary pass picks one.
func successResponses(responses *v3high.Responses) []*v3high.Response {
	var found []*v3high.Response
	if responses.Codes != nil {
		for statusCode, response := range responses.Codes.FromOldest() {
			status, err := strconv.Atoi(statusCode)
			if err != nil {
				if strings.EqualFold(statusCode, "2xx") {
					found = append(found, response)
				}
				continue
			}
			if status >= 200 && status < 300 {
				found = append(found, response)
			}
		}
	}
	if len(found) == 0 && responses.Default != nil {
		found = append(found, responses.Default)
	}
	return found
}

// classifyMediaTypes reports whether a response declares a sequential media type
// and whether anything else sits alongside it.
func classifyMediaTypes(content *orderedmap.Map[string, *v3high.MediaType]) (sequential, other bool) {
	for mediaType := range content.KeysFromOldest() {
		if _, ok := streamFraming(mediaType); ok {
			sequential = true
			continue
		}
		other = true
	}
	return sequential, other
}

// collectStreamResponses resolves each success response's sequential media type
// into a StreamResponseDefinition, in ascending status order.
//
// It looks at every media type at a status, not the one the primary pass
// selected, so an operation offering both `application/json` and
// `text/event-stream` yields both shapes. Nothing here changes the primary.
func collectStreamResponses(
	operationID string,
	contents map[int]*orderedmap.Map[string, *v3high.MediaType],
	fallbacks map[int]streamFallback,
	options ParseOptions,
) ([]*StreamResponseDefinition, []TypeDefinition, error) {
	// Opt-in: with the flag off no item types are generated and no stream
	// definitions exist, so nothing downstream changes.
	if !options.ClientStreaming || len(contents) == 0 {
		return nil, nil, nil
	}

	statuses := make([]int, 0, len(contents))
	for status := range contents {
		statuses = append(statuses, status)
	}
	sort.Ints(statuses)

	var (
		streams         []*StreamResponseDefinition
		typeDefinitions []TypeDefinition
	)

	for _, status := range statuses {
		contentType, content := findSequentialMediaType(contents[status])
		if content == nil {
			continue
		}

		framing, _ := streamFraming(contentType)
		fallback := fallbacks[status]
		itemName, itemTypes, err := streamItemType(content, fallback.proxy, fallback.schema, operationID, options)
		if err != nil {
			return nil, nil, err
		}
		typeDefinitions = append(typeDefinitions, itemTypes...)

		streams = append(streams, &StreamResponseDefinition{
			StatusCode:  status,
			ContentType: contentType,
			Framing:     framing,
			ItemName:    itemName,
		})
	}

	return streams, typeDefinitions, nil
}

// findSequentialMediaType returns the first sequential media type in spec order.
func findSequentialMediaType(content *orderedmap.Map[string, *v3high.MediaType]) (string, *v3high.MediaType) {
	if content == nil {
		return "", nil
	}
	for mediaType, value := range content.FromOldest() {
		if _, ok := streamFraming(mediaType); ok {
			return mediaType, value
		}
	}
	return "", nil
}

// streamItemType resolves the Go type of a single frame, plus any type
// definitions generated for it.
//
// The OpenAPI 3.2 `itemSchema` wins; otherwise `schema` is used, which is how
// 3.0 and 3.1 specs describe an event payload. A $ref resolves to the
// already-generated component, an inline object becomes
// <OperationID>ResponseItem, and an undecodable schema yields raw bytes.
//
// bodySchema is what the caller generated fallback from; when it is the schema
// we would pick anyway, fallback is reused.
func streamItemType(content *v3high.MediaType, bodySchema *base.SchemaProxy, fallback GoSchema, operationID string, options ParseOptions) (string, []TypeDefinition, error) {
	if content == nil {
		return streamRawItemType, nil, nil
	}

	itemSchema := content.ItemSchema
	if itemSchema == nil {
		itemSchema = content.Schema
	}
	if itemSchema == nil {
		return streamRawItemType, nil, nil
	}

	// The fallback was generated with the reference cleared, so it copies the
	// component's schema rather than pointing at it. Recover the name.
	if ref := itemSchema.GetReference(); ref != "" {
		return streamItemTypeFromRef(ref, options)
	}

	if itemSchema == bodySchema {
		return registerStreamItemType(fallback, operationID, options)
	}

	return streamItemTypeFromSchema(itemSchema, operationID, options)
}

// streamItemTypeFromSchema resolves an explicit OpenAPI 3.2 itemSchema.
func streamItemTypeFromSchema(itemSchema *base.SchemaProxy, operationID string, options ParseOptions) (string, []TypeDefinition, error) {
	if ref := itemSchema.GetReference(); ref != "" {
		return streamItemTypeFromRef(ref, options)
	}

	opts := options.
		WithReference("").
		WithPath([]string{operationID, "ResponseItem"}).
		WithSpecLocation(SpecLocationResponse)
	itemGoSchema, err := GenerateGoSchema(itemSchema, opts)
	if err != nil {
		return "", nil, fmt.Errorf("error generating stream item definition: %w", err)
	}

	return registerStreamItemType(itemGoSchema, operationID, opts)
}

// streamItemTypeFromRef maps a $ref to the component's generated Go type,
// honouring any renaming the type tracker applied.
func streamItemTypeFromRef(ref string, options ParseOptions) (string, []TypeDefinition, error) {
	if name, ok := options.typeTracker.LookupByRef(ref); ok {
		return name, nil, nil
	}

	goType, err := refPathToGoType(ref)
	if err != nil {
		return "", nil, fmt.Errorf("error turning stream item reference (%s) into a Go type: %w", ref, err)
	}
	return goType, nil, nil
}

// registerStreamItemType registers an inline item schema as
// <OperationID>ResponseItem.
func registerStreamItemType(itemGoSchema GoSchema, operationID string, options ParseOptions) (string, []TypeDefinition, error) {
	if !isDecodableStreamItem(itemGoSchema) {
		return streamRawItemType, nil, nil
	}

	if itemGoSchema.ArrayType != nil {
		itemGoSchema, _ = replaceInlineTypes(itemGoSchema, options)
	}

	name := options.typeTracker.generateUniqueName(operationID + "ResponseItem")
	td := TypeDefinition{
		Name:           name,
		Schema:         itemGoSchema,
		SpecLocation:   SpecLocationResponse,
		NeedsMarshaler: needsMarshaler(itemGoSchema),
	}
	options.typeTracker.register(td, "")

	typeDefinitions := []TypeDefinition{td}
	for _, additionalType := range itemGoSchema.AdditionalTypes {
		if _, exists := options.typeTracker.LookupByName(additionalType.Name); !exists {
			typeDefinitions = append(typeDefinitions, additionalType)
			options.typeTracker.register(additionalType, "")
		}
	}

	return name, typeDefinitions, nil
}

// isDecodableStreamItem reports whether a frame payload can be JSON-decoded into
// the schema's Go type. A bare string cannot: frames carry unquoted text.
func isDecodableStreamItem(schema GoSchema) bool {
	if schema.IsZero() {
		return false
	}
	switch schema.GoType {
	case "", "string", "[]byte", "any", "interface{}":
		return false
	}
	return true
}
