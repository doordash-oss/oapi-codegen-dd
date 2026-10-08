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
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadOperation builds a model from an inline spec and returns one operation
// with ParseOptions wired to it.
func loadOperation(t *testing.T, contents []byte, path, method string) (*v3.Operation, ParseOptions) {
	t.Helper()
	doc, err := libopenapi.NewDocument(contents)
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.Empty(t, errs)

	item := model.Model.Paths.PathItems.GetOrZero(path)
	require.NotNil(t, item, "path %s not found", path)

	var op *v3.Operation
	switch method {
	case "get":
		op = item.Get
	case "post":
		op = item.Post
	case "put":
		op = item.Put
	case "delete":
		op = item.Delete
	}
	require.NotNil(t, op, "operation %s %s not found", method, path)

	opts := ParseOptions{
		typeTracker: newTypeTracker(),
		visited:     map[string]bool{},
		model:       &model.Model,
	}
	return op, opts
}

func TestGetOperationResponses(t *testing.T) {
	t.Run("default response with content becomes 200 success when no explicit success documented", func(t *testing.T) {
		contents := []byte(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Test
paths:
  /ping:
    get:
      operationId: ping
      responses:
        default:
          description: any
          content:
            application/json:
              schema:
                type: object
                properties:
                  message:
                    type: string
`)
		op, opts := loadOperation(t, contents, "/ping", "get")
		res, _, err := getOperationResponses("ping", op.Responses, opts)
		require.NoError(t, err)
		require.NotNil(t, res)

		assert.Equal(t, 200, res.SuccessStatusCode,
			"default with content must be installed as 200 success, not a fabricated 204")
		require.NotNil(t, res.Success)
		assert.True(t, res.Success.IsSuccess)
		assert.Equal(t, "application/json", res.Success.ContentType)
		assert.False(t, res.Success.Schema.IsZero(),
			"the default response's schema must drive the success type")

		// Default must NOT also be installed as a 500 error in this case.
		_, has500 := res.All[500]
		assert.False(t, has500, "default was promoted to success; it must not also appear as 500")

		// No fabricated 204.
		_, has204 := res.All[204]
		assert.False(t, has204)
	})

	t.Run("default response stays as 500 error when explicit success is documented", func(t *testing.T) {
		contents := []byte(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Test
paths:
  /ping:
    get:
      operationId: ping
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                type: object
                properties:
                  ok:
                    type: boolean
        default:
          description: error
          content:
            application/json:
              schema:
                type: object
                properties:
                  message:
                    type: string
`)
		op, opts := loadOperation(t, contents, "/ping", "get")
		res, _, err := getOperationResponses("ping", op.Responses, opts)
		require.NoError(t, err)

		assert.Equal(t, 200, res.SuccessStatusCode)
		require.NotNil(t, res.Success)
		assert.True(t, res.Success.IsSuccess)

		require.NotNil(t, res.Error, "default must be installed as the fallback error")
		assert.Equal(t, 500, res.Error.StatusCode)
		assert.False(t, res.Error.IsSuccess)
	})

	t.Run("default response without content falls back to fabricated 204 success", func(t *testing.T) {
		contents := []byte(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Test
paths:
  /ping:
    get:
      operationId: ping
      responses:
        default:
          description: any
`)
		op, opts := loadOperation(t, contents, "/ping", "get")
		res, _, err := getOperationResponses("ping", op.Responses, opts)
		require.NoError(t, err)

		// No content on default means there's nothing to promote to success;
		// the 204 fallback kicks in.
		assert.Equal(t, 204, res.SuccessStatusCode)
		require.NotNil(t, res.Success)
		assert.Equal(t, "struct{}", res.Success.ResponseName)
	})

	t.Run("empty success body preserves declared ContentType", func(t *testing.T) {
		contents := []byte(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Test
paths:
  /page:
    get:
      operationId: getPage
      responses:
        '200':
          description: html
          content:
            text/html: {}
`)
		op, opts := loadOperation(t, contents, "/page", "get")
		res, _, err := getOperationResponses("getPage", op.Responses, opts)
		require.NoError(t, err)

		require.NotNil(t, res.Success)
		assert.Equal(t, 200, res.Success.StatusCode)
		assert.Equal(t, "struct{}", res.Success.ResponseName,
			"no schema = no decoded body, so the response is struct{}")
		assert.Equal(t, "text/html", res.Success.ContentType,
			"the declared media type must survive even when the body schema is empty")
	})

	t.Run("default with content promoted to success generates a Response type, not an ErrorResponse", func(t *testing.T) {
		contents := []byte(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Test
paths:
  /ping:
    get:
      operationId: ping
      responses:
        default:
          description: any
          content:
            application/json:
              schema:
                type: object
                properties:
                  message:
                    type: string
`)
		op, opts := loadOperation(t, contents, "/ping", "get")
		res, typeDefs, err := getOperationResponses("ping", op.Responses, opts)
		require.NoError(t, err)
		require.NotNil(t, res.Success)

		// The success-side name suffix is "Response", not "ErrorResponse".
		assert.Equal(t, "pingResponse", res.Success.ResponseName)

		var names []string
		for _, td := range typeDefs {
			names = append(names, td.Name)
		}
		assert.Contains(t, names, "pingResponse")
		assert.NotContains(t, names, "pingErrorResponse",
			"when default is promoted to success, no parallel ErrorResponse type should be emitted")
	})
}

func TestGetOperationResponsesEnvelope(t *testing.T) {
	const spec = `
openapi: "3.1.0"
info:
  version: 1.0.0
  title: Test
paths:
  /ranges:
    get:
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Thing'
        '4XX':
          description: client error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ClientError'
        '5XX':
          description: server error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ServerError'
  /mixed:
    get:
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Thing'
        '404':
          description: not found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ClientError'
        default:
          description: anything else
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ServerError'
  /bodyless:
    get:
      responses:
        '200':
          description: ok
        '404':
          description: not found, no body
        '503':
          description: unavailable, no body
        '4XX':
          description: client error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ClientError'
        '5XX':
          description: server error, no body
  /bodyless-range:
    get:
      responses:
        '200':
          description: ok
        '5XX':
          description: server error, no body
        default:
          description: anything else
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ServerError'
  /component-ref:
    get:
      responses:
        '200':
          description: ok
        '4XX':
          $ref: '#/components/responses/ClientError'
  /default-only:
    get:
      responses:
        default:
          description: anything
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Thing'
  /exact-and-range:
    get:
      responses:
        '200':
          description: ok
        '400':
          description: bad request
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ClientError'
        '404':
          description: not found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ClientError'
        '4XX':
          description: any other client error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ServerError'
  /success-range:
    get:
      responses:
        '2XX':
          description: any success
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Thing'
  /default-headers:
    get:
      responses:
        '200':
          description: ok
        default:
          description: any error
          headers:
            X-Request-Id:
              schema:
                type: string
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ServerError'
  /default-headers-alongside:
    get:
      responses:
        '200':
          description: ok
        '404':
          description: not found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ClientError'
        default:
          description: any other error
          headers:
            X-Request-Id:
              schema:
                type: string
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ServerError'
components:
  responses:
    ClientError:
      description: client error
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ClientError'
  schemas:
    Thing:
      type: object
      properties:
        id:
          type: string
    ClientError:
      type: object
      properties:
        message:
          type: string
    ServerError:
      type: object
      properties:
        reason:
          type: string
`

	withEnvelope := func(opts ParseOptions) ParseOptions {
		opts.ClientWithResponse = true
		return opts
	}

	// The pipeline registers component types before operations, which is how
	// a response referencing a component response finds its type.
	withComponents := func(t *testing.T, opts ParseOptions) ParseOptions {
		t.Helper()
		_, err := collectComponentDefinitions(opts.model, opts)
		require.NoError(t, err)
		return opts
	}

	t.Run("4XX and 5XX keep their own entries and types", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/ranges", "get")
		def, typeDefs, err := getOperationResponses("GetRanges", op.Responses, withEnvelope(opts))
		require.NoError(t, err)

		assert.Empty(t, def.Errors, "neither range is an exact code")
		require.Len(t, def.StatusRanges, 2)
		clientErr, serverErr := def.StatusRanges[0], def.StatusRanges[1]

		assert.Equal(t, 4, clientErr.StatusRange)
		assert.Equal(t, "4XX", clientErr.StatusName())
		assert.Equal(t, "JSON", clientErr.NameTag)
		assert.Equal(t, 5, serverErr.StatusRange)
		assert.Equal(t, "5XX", serverErr.StatusName())

		// Both ranges once landed on 400, where 5XX replaced 4XX. Each must
		// decode into its own schema: ClientError has "message", ServerError
		// has "reason".
		fields := map[string]string{}
		for _, td := range typeDefs {
			if len(td.Schema.Properties) == 1 {
				fields[td.Name] = td.Schema.Properties[0].JsonFieldName
			}
		}
		assert.Equal(t, "message", fields[clientErr.ResponseName])
		assert.Equal(t, "reason", fields[serverErr.ResponseName])
	})

	t.Run("default next to explicit errors is resolved for the envelope client", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/mixed", "get")
		def, typeDefs, err := getOperationResponses("GetMixed", op.Responses, withEnvelope(opts))
		require.NoError(t, err)

		require.NotNil(t, def.Default)
		assert.True(t, def.Default.IsDefault)
		assert.False(t, def.Default.IsSuccess)
		assert.Equal(t, "JSON", def.Default.NameTag)
		assert.Equal(t, "Default", def.Default.StatusName())
		assert.NotEqual(t, def.Errors[0].ResponseName, def.Default.ResponseName,
			"default must not reuse the name the 404 response owns")

		var names []string
		for _, td := range typeDefs {
			names = append(names, td.Name)
		}
		assert.Contains(t, names, def.Default.ResponseName)

		// Handler generation still answers errors with the 404.
		require.NotNil(t, def.Error)
		assert.Equal(t, 404, def.Error.StatusCode)
		_, has500 := def.All[500]
		assert.False(t, has500)
	})

	t.Run("default next to explicit errors is skipped without the envelope client", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/mixed", "get")
		def, typeDefs, err := getOperationResponses("GetMixed", op.Responses, opts)
		require.NoError(t, err)
		assert.Nil(t, def.Default)

		op, opts = loadOperation(t, []byte(spec), "/mixed", "get")
		_, envelopeTypeDefs, err := getOperationResponses("GetMixed", op.Responses, withEnvelope(opts))
		require.NoError(t, err)
		assert.Len(t, envelopeTypeDefs, len(typeDefs)+1, "only the envelope client gets a type for default")
	})

	t.Run("an exact code without a body keeps precedence over the range covering it", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/bodyless", "get")
		def, _, err := getOperationResponses("GetBodyless", op.Responses, withEnvelope(opts))
		require.NoError(t, err)

		// 404 gets its own case so 4XX does not decode it. Nothing with a body
		// covers 503 or the bodyless 5XX, so both stay with the
		// unexpected-status fallback.
		require.Len(t, def.Errors, 1)
		assert.Equal(t, 404, def.Errors[0].StatusCode)
		assert.Equal(t, "struct{}", def.Errors[0].ResponseName)
		require.Len(t, def.StatusRanges, 1)
		assert.Equal(t, 4, def.StatusRanges[0].StatusRange)

		// The handler-facing view never held bodyless errors.
		_, has404 := def.All[404]
		assert.False(t, has404)
	})

	t.Run("a range without a body keeps precedence over default", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/bodyless-range", "get")
		def, _, err := getOperationResponses("GetBodylessRange", op.Responses, withEnvelope(opts))
		require.NoError(t, err)

		require.Len(t, def.StatusRanges, 1)
		assert.Equal(t, 5, def.StatusRanges[0].StatusRange)
		assert.Equal(t, "struct{}", def.StatusRanges[0].ResponseName)
		assert.NotNil(t, def.Default)
	})

	t.Run("a range referencing a component response gets a typed field", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/component-ref", "get")
		def, _, err := getOperationResponses("GetComponentRef", op.Responses, withEnvelope(withComponents(t, opts)))
		require.NoError(t, err)

		require.Len(t, def.StatusRanges, 1)
		assert.True(t, def.StatusRanges[0].componentResponse, "the component response's type must be used")
		assert.Equal(t, "JSON", def.StatusRanges[0].NameTag)
	})

	t.Run("default standing in for the success is the envelope's Default", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/default-only", "get")
		def, _, err := getOperationResponses("GetDefaultOnly", op.Responses, withEnvelope(opts))
		require.NoError(t, err)

		require.NotNil(t, def.Default)
		assert.Same(t, def.Success, def.Default)
		assert.True(t, def.Default.IsSuccess)
		assert.Empty(t, def.Successes, "default is matched last, not as an exact 200")
	})

	t.Run("an exact code and the range covering it sit alongside, names included", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/exact-and-range", "get")
		def, _, err := getOperationResponses("GetExactAndRange", op.Responses, withEnvelope(opts))
		require.NoError(t, err)

		require.Len(t, def.Errors, 2)
		badRequest := def.Errors[0]
		assert.Equal(t, 400, badRequest.StatusCode)
		require.Len(t, def.StatusRanges, 1)
		clientErr := def.StatusRanges[0]

		// A type name suffixed with a status takes the range, so 400 stays
		// the explicit code's.
		assert.Equal(t, "GetExactAndRangeErrorResponseJSON4XX", clientErr.ResponseName)
		assert.NotContains(t, clientErr.ResponseName, "400")

		// 4XX overwrote 400 in All, but the explicit 400 owns JSON400, so the
		// range gets no deprecated alias under that name.
		assert.Same(t, clientErr, def.All[400])
		assert.Empty(t, def.LegacyBodyField(clientErr))
	})

	t.Run("a range keeps the field it had at its stand-in code as an alias", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/success-range", "get")
		def, _, err := getOperationResponses("GetSuccessRange", op.Responses, withEnvelope(opts))
		require.NoError(t, err)

		require.Len(t, def.StatusRanges, 1)
		assert.Equal(t, "JSON200", def.LegacyBodyField(def.StatusRanges[0]))
	})

	t.Run("of 4XX and 5XX, the one the old field held keeps it", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/ranges", "get")
		def, _, err := getOperationResponses("GetRanges", op.Responses, withEnvelope(opts))
		require.NoError(t, err)

		// 5XX came last, so the old JSON400 field had its type.
		clientErr, serverErr := def.StatusRanges[0], def.StatusRanges[1]
		assert.Empty(t, def.LegacyBodyField(clientErr))
		assert.Equal(t, "JSON400", def.LegacyBodyField(serverErr))
	})

	t.Run("a range referencing a component response had no field to keep", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/component-ref", "get")
		def, _, err := getOperationResponses("GetComponentRef", op.Responses, withEnvelope(withComponents(t, opts)))
		require.NoError(t, err)

		assert.Empty(t, def.LegacyBodyField(def.StatusRanges[0]))
	})

	t.Run("default keeps its headers under the code it stood in at", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/default-headers", "get")
		def, _, err := getOperationResponses("GetDefaultHeaders", op.Responses, withEnvelope(opts))
		require.NoError(t, err)

		require.NotNil(t, def.Default)
		assert.True(t, def.HasLegacyHeaders(def.Default))
		assert.Empty(t, def.LegacyBodyField(def.Default), "default never had a body field")
	})

	t.Run("default next to explicit errors was never in the envelope", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/default-headers-alongside", "get")
		def, _, err := getOperationResponses("GetDefaultHeadersAlongside", op.Responses, withEnvelope(opts))
		require.NoError(t, err)

		require.NotNil(t, def.Default)
		require.NotEmpty(t, def.Default.Headers)
		assert.False(t, def.HasLegacyHeaders(def.Default), "the old envelope dropped it, so there is nothing to keep")
	})
}

// Bundled specs point a response at another operation's inline schema with a
// JSON pointer. That names no generated type, so default gets its own.
func TestGetOperationResponsesDefaultPointer(t *testing.T) {
	const spec = `
openapi: "3.0.3"
info:
  version: 1.0.0
  title: Test
paths:
  /errors:
    get:
      responses:
        '200':
          description: ok
        '401':
          description: unauthorized
          content:
            application/json:
              schema:
                type: object
                required: [id]
                properties:
                  id:
                    type: string
        default:
          description: anything else
          content:
            application/json:
              schema:
                $ref: '#/paths/~1errors/get/responses/401/content/application~1json/schema'
  /default-only:
    get:
      responses:
        '200':
          description: ok
        default:
          description: any error
          content:
            application/json:
              schema:
                $ref: '#/paths/~1errors/get/responses/401/content/application~1json/schema'
`

	requireInline := func(t *testing.T, rcd *ResponseContentDefinition, typeDefs []TypeDefinition) {
		t.Helper()
		require.NotNil(t, rcd)
		assert.Empty(t, rcd.Ref)
		for _, td := range typeDefs {
			if td.Name != rcd.ResponseName {
				continue
			}
			assert.Empty(t, td.Schema.RefType, "a JSON pointer must not become a reference to an ungenerated type")
			require.Len(t, td.Schema.Properties, 1)
			assert.Equal(t, "id", td.Schema.Properties[0].JsonFieldName)
			return
		}
		t.Fatalf("no type definition for %s", rcd.ResponseName)
	}

	t.Run("next to explicit errors", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/errors", "get")
		opts.ClientWithResponse = true
		def, typeDefs, err := getOperationResponses("GetErrors", op.Responses, opts)
		require.NoError(t, err)
		requireInline(t, def.Default, typeDefs)
	})

	t.Run("as the only error", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/default-only", "get")
		def, typeDefs, err := getOperationResponses("GetDefaultOnly", op.Responses, opts)
		require.NoError(t, err)
		assert.Same(t, def.Error, def.Default)
		requireInline(t, def.Default, typeDefs)
	})
}

func TestStatusRangeOf(t *testing.T) {
	tests := []struct {
		key      string
		expected int
	}{
		{key: "1XX", expected: 1},
		{key: "2XX", expected: 2},
		{key: "4XX", expected: 4},
		{key: "5xx", expected: 5},
		{key: "6XX"},
		{key: "0XX"},
		{key: "404"},
		{key: "40X"},
		{key: "4X"},
		{key: "default"},
		{key: ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			assert.Equal(t, tt.expected, statusRangeOf(tt.key))
		})
	}
}

func TestResponseNameTag(t *testing.T) {
	tests := map[string]string{
		"application/json":                  "JSON",
		"application/problem+json":          "ApplicationProblemPlusJSON",
		"application/x-www-form-urlencoded": "Formdata",
		"multipart/form-data":               "Multipart",
		"text/plain":                        "Text",
		"text/html":                         "HTML",
		"application/octet-stream":          "",
		"application/xml":                   "",
	}

	for contentType, expected := range tests {
		t.Run(contentType, func(t *testing.T) {
			assert.Equal(t, expected, responseNameTag(contentType))
		})
	}
}

func TestResponseDefinitionEnvelopeAccessors(t *testing.T) {
	ok := &ResponseContentDefinition{StatusCode: 200, IsSuccess: true}
	notFound := &ResponseContentDefinition{StatusCode: 404}
	clientErr := &ResponseContentDefinition{StatusCode: 400, StatusRange: 4}
	fallback := &ResponseContentDefinition{StatusCode: 500, IsDefault: true}

	t.Run("StatusName", func(t *testing.T) {
		assert.Equal(t, "200", ok.StatusName())
		assert.Equal(t, "4XX", clientErr.StatusName())
		assert.Equal(t, "Default", fallback.StatusName())
	})

	t.Run("EnvelopeResponses lists responses in match order", func(t *testing.T) {
		def := ResponseDefinition{
			Successes:    []*ResponseContentDefinition{ok},
			Errors:       []*ResponseContentDefinition{notFound},
			StatusRanges: []*ResponseContentDefinition{clientErr},
			Default:      fallback,
		}
		assert.Equal(t, []*ResponseContentDefinition{ok, notFound, clientErr, fallback}, def.EnvelopeResponses())

		def.Default = nil
		assert.Equal(t, []*ResponseContentDefinition{ok, notFound, clientErr}, def.EnvelopeResponses())
	})

	t.Run("StreamFor matches only the response All holds at the stream's code", func(t *testing.T) {
		stream := &StreamResponseDefinition{StatusCode: 200, ContentType: "text/event-stream", Framing: streamFramingSSE, ItemName: "Event"}
		successRange := &ResponseContentDefinition{StatusCode: 200, StatusRange: 2, IsSuccess: true}
		def := ResponseDefinition{
			All:     map[int]*ResponseContentDefinition{200: ok},
			Streams: []*StreamResponseDefinition{stream},
		}

		assert.Same(t, stream, def.StreamFor(ok))
		assert.Nil(t, def.StreamFor(successRange), "2XX shares 200 with the exact code, which owns the stream")
		assert.Nil(t, def.StreamFor(notFound))
	})
}

func TestResponseDefinitionStreamAccessors(t *testing.T) {
	sse := &StreamResponseDefinition{StatusCode: 200, ContentType: "text/event-stream", Framing: streamFramingSSE, ItemName: "Event"}
	lines := &StreamResponseDefinition{StatusCode: 206, ContentType: "application/jsonl", Framing: streamFramingLines, ItemName: "Row"}

	t.Run("without streams", func(t *testing.T) {
		var def ResponseDefinition
		assert.False(t, def.HasStream())
		assert.Nil(t, def.PrimaryStream())
		assert.Nil(t, def.StreamAt(200))
	})

	t.Run("with streams", func(t *testing.T) {
		def := ResponseDefinition{Streams: []*StreamResponseDefinition{sse, lines}}

		assert.True(t, def.HasStream())
		// The lowest success status is what the sibling method streams.
		assert.Same(t, sse, def.PrimaryStream())
		assert.Same(t, lines, def.StreamAt(206))
		assert.Nil(t, def.StreamAt(404))
	})
}

func TestResponseBodySchema(t *testing.T) {
	schema := base.CreateSchemaProxy(&base.Schema{Type: []string{"object"}})
	itemSchema := base.CreateSchemaProxy(&base.Schema{Type: []string{"object"}})

	tests := []struct {
		name        string
		content     *v3.MediaType
		contentType string
		expected    *base.SchemaProxy
	}{
		{name: "nil content", contentType: "application/json"},
		{
			name:        "schema wins when present",
			content:     &v3.MediaType{Schema: schema, ItemSchema: itemSchema},
			contentType: "text/event-stream",
			expected:    schema,
		},
		{
			name:        "itemSchema stands in for a sequential media type",
			content:     &v3.MediaType{ItemSchema: itemSchema},
			contentType: "text/event-stream",
			expected:    itemSchema,
		},
		{
			name:        "itemSchema is ignored for a buffered media type",
			content:     &v3.MediaType{ItemSchema: itemSchema},
			contentType: "application/json",
		},
		{
			name:        "no schema at all",
			content:     &v3.MediaType{},
			contentType: "text/event-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Same(t, tt.expected, responseBodySchema(tt.content, tt.contentType))
		})
	}
}

func TestGetOperationResponsesStreams(t *testing.T) {
	const spec = `
openapi: "3.2.0"
info:
  version: 1.0.0
  title: Test
paths:
  /events:
    get:
      responses:
        '200':
          description: sequential only
          content:
            text/event-stream:
              schema:
                $ref: '#/components/schemas/Event'
  /chat:
    get:
      responses:
        '200':
          description: buffered and sequential together
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Completion'
            text/event-stream:
              schema:
                $ref: '#/components/schemas/Event'
  /feed:
    get:
      responses:
        default:
          description: sequential under default only
          content:
            text/event-stream:
              schema:
                $ref: '#/components/schemas/Event'
  /items:
    get:
      responses:
        '200':
          description: itemSchema without a whole-body schema
          content:
            application/jsonl:
              itemSchema:
                $ref: '#/components/schemas/Event'
  /users:
    get:
      responses:
        '200':
          description: buffered only
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Completion'
components:
  schemas:
    Event:
      type: object
      properties:
        seq:
          type: integer
    Completion:
      type: object
      properties:
        text:
          type: string
`

	streamingOptions := func(opts ParseOptions) ParseOptions {
		opts.ClientStreaming = true
		return opts
	}

	t.Run("a sequential-only response is recorded and marks the selected media type", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/events", "get")
		def, _, err := getOperationResponses("GetEvents", op.Responses, streamingOptions(opts))
		require.NoError(t, err)

		require.True(t, def.HasStream())
		require.Len(t, def.Streams, 1)
		assert.Equal(t, 200, def.Streams[0].StatusCode)
		assert.Equal(t, "text/event-stream", def.Streams[0].ContentType)
		assert.Equal(t, streamFramingSSE, def.Streams[0].Framing)
		assert.Equal(t, "Event", def.Streams[0].ItemName)

		// The primary method selected the sequential media type, so it buffers
		// and blocks - which is what IsStream exists to signal.
		assert.True(t, def.Success.IsStream)
		assert.True(t, def.Success.IsRaw, "the whole-body type stays []byte for handler generation")
	})

	t.Run("a buffered alternative keeps the primary response and still records the stream", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/chat", "get")
		def, _, err := getOperationResponses("Chat", op.Responses, streamingOptions(opts))
		require.NoError(t, err)

		require.Len(t, def.Streams, 1)
		assert.Equal(t, "text/event-stream", def.Streams[0].ContentType)
		assert.Equal(t, "Event", def.Streams[0].ItemName)

		// JSON is what the primary method returns, so it never blocks.
		assert.Equal(t, "application/json", def.Success.ContentType)
		assert.False(t, def.Success.IsStream)
		assert.False(t, def.Success.IsRaw)
	})

	t.Run("a default response standing in as the success is recorded", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/feed", "get")
		def, _, err := getOperationResponses("Feed", op.Responses, streamingOptions(opts))
		require.NoError(t, err)

		require.Len(t, def.Streams, 1)
		assert.Equal(t, 200, def.Streams[0].StatusCode)
		assert.Equal(t, "Event", def.Streams[0].ItemName)
	})

	t.Run("itemSchema alone still counts as content", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/items", "get")
		def, _, err := getOperationResponses("Items", op.Responses, streamingOptions(opts))
		require.NoError(t, err)

		require.Len(t, def.Streams, 1)
		assert.Equal(t, streamFramingLines, def.Streams[0].Framing)
		assert.Equal(t, "Event", def.Streams[0].ItemName)
		assert.NotEqual(t, "struct{}", def.Success.ResponseName, "the response must not be mistaken for empty")
	})

	t.Run("a buffered-only response records nothing", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/users", "get")
		def, _, err := getOperationResponses("ListUsers", op.Responses, streamingOptions(opts))
		require.NoError(t, err)

		assert.False(t, def.HasStream())
		assert.False(t, def.Success.IsStream)
	})

	t.Run("detection is off by default", func(t *testing.T) {
		op, opts := loadOperation(t, []byte(spec), "/events", "get")
		def, types, err := getOperationResponses("GetEvents", op.Responses, opts)
		require.NoError(t, err)

		assert.False(t, def.HasStream(), "no stream definitions without the flag")
		for _, td := range types {
			assert.NotContains(t, td.Name, "ResponseItem", "no per-frame item types without the flag")
		}

		// IsStream is media-type derived, so it holds either way - the MCP tool
		// template relies on it regardless of the flag.
		assert.True(t, def.Success.IsStream)
	})
}
