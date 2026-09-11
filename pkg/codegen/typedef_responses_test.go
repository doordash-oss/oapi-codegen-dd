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
