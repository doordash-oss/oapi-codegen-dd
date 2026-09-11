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

	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadStreamingModel builds the v3 model for a spec in testdata.
func loadStreamingModel(t *testing.T, name string) *v3high.Document {
	t.Helper()
	doc, err := LoadDocumentFromContents([]byte(readTestdata(t, name)))
	require.NoError(t, err)
	model, err := doc.BuildV3Model()
	require.NoError(t, err)
	return &model.Model
}

// streamOptions returns ParseOptions with detection enabled.
func streamOptions() ParseOptions {
	return ParseOptions{
		ClientStreaming: true,
		typeTracker:     newTypeTracker(),
		visited:         map[string]bool{},
	}
}

// mediaTypes builds an ordered content map from media type names.
func mediaTypes(names ...string) *orderedmap.Map[string, *v3high.MediaType] {
	content := orderedmap.New[string, *v3high.MediaType]()
	for _, name := range names {
		content.Set(name, &v3high.MediaType{})
	}
	return content
}

func TestStreamFraming(t *testing.T) {
	tests := []struct {
		name            string
		contentType     string
		expectedFraming string
		expectedOK      bool
	}{
		{name: "server-sent events", contentType: "text/event-stream", expectedFraming: streamFramingSSE, expectedOK: true},
		{name: "server-sent events with charset", contentType: "text/event-stream; charset=utf-8", expectedFraming: streamFramingSSE, expectedOK: true},
		{name: "unparseable parameters fall back to the prefix", contentType: "text/event-stream; charset", expectedFraming: streamFramingSSE, expectedOK: true},
		{name: "uppercase is normalized", contentType: "TEXT/EVENT-STREAM", expectedFraming: streamFramingSSE, expectedOK: true},
		{name: "ndjson", contentType: "application/x-ndjson", expectedFraming: streamFramingLines, expectedOK: true},
		{name: "ndjson without prefix", contentType: "application/ndjson", expectedFraming: streamFramingLines, expectedOK: true},
		{name: "jsonl", contentType: "application/jsonl", expectedFraming: streamFramingLines, expectedOK: true},
		{name: "jsonlines", contentType: "application/x-jsonlines", expectedFraming: streamFramingLines, expectedOK: true},
		{name: "json-lines", contentType: "application/json-lines", expectedFraming: streamFramingLines, expectedOK: true},
		{name: "plain json is not sequential", contentType: "application/json"},
		{name: "stream+json is treated as plain json", contentType: "application/stream+json"},
		{name: "json sequences are not supported", contentType: "application/json-seq"},
		{name: "multipart is not supported", contentType: "multipart/mixed"},
		{name: "empty", contentType: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			framing, ok := streamFraming(tt.contentType)
			assert.Equal(t, tt.expectedOK, ok)
			assert.Equal(t, tt.expectedFraming, framing)
		})
	}
}

func TestIsDecodableStreamItem(t *testing.T) {
	tests := []struct {
		name     string
		schema   GoSchema
		expected bool
	}{
		{name: "struct", schema: GoSchema{GoType: "Event"}, expected: true},
		{name: "array", schema: GoSchema{GoType: "[]Event"}, expected: true},
		{name: "integer", schema: GoSchema{GoType: "int"}, expected: true},
		{name: "zero schema", schema: GoSchema{}},
		{name: "bare string carries unquoted text", schema: GoSchema{GoType: "string"}},
		{name: "bytes", schema: GoSchema{GoType: "[]byte"}},
		{name: "any", schema: GoSchema{GoType: "any"}},
		{name: "empty interface", schema: GoSchema{GoType: "interface{}"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isDecodableStreamItem(tt.schema))
		})
	}
}

func TestClassifyMediaTypes(t *testing.T) {
	tests := []struct {
		name               string
		names              []string
		expectedSequential bool
		expectedOther      bool
	}{
		{name: "sequential only", names: []string{"text/event-stream"}, expectedSequential: true},
		{name: "sequential alongside json", names: []string{"application/json", "text/event-stream"}, expectedSequential: true, expectedOther: true},
		{name: "buffered only", names: []string{"application/json"}, expectedOther: true},
		{name: "empty", names: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sequential, other := classifyMediaTypes(mediaTypes(tt.names...))
			assert.Equal(t, tt.expectedSequential, sequential)
			assert.Equal(t, tt.expectedOther, other)
		})
	}
}

func TestFindSequentialMediaType(t *testing.T) {
	t.Run("returns the first sequential entry in spec order", func(t *testing.T) {
		contentType, content := findSequentialMediaType(mediaTypes("application/json", "application/x-ndjson", "text/event-stream"))
		assert.Equal(t, "application/x-ndjson", contentType)
		assert.NotNil(t, content)
	})

	t.Run("returns nothing when no entry is sequential", func(t *testing.T) {
		contentType, content := findSequentialMediaType(mediaTypes("application/json", "text/csv"))
		assert.Empty(t, contentType)
		assert.Nil(t, content)
	})

	t.Run("tolerates nil content", func(t *testing.T) {
		contentType, content := findSequentialMediaType(nil)
		assert.Empty(t, contentType)
		assert.Nil(t, content)
	})
}

func TestSuccessResponses(t *testing.T) {
	responseFor := func(codes map[string]*v3high.Response, def *v3high.Response) *v3high.Responses {
		responses := &v3high.Responses{Default: def}
		if codes != nil {
			responses.Codes = orderedmap.New[string, *v3high.Response]()
			for code, response := range codes {
				responses.Codes.Set(code, response)
			}
		}
		return responses
	}

	success := &v3high.Response{Description: "ok"}
	failure := &v3high.Response{Description: "boom"}
	fallback := &v3high.Response{Description: "default"}

	t.Run("collects 2xx codes", func(t *testing.T) {
		found := successResponses(responseFor(map[string]*v3high.Response{"200": success, "500": failure}, nil))
		assert.Equal(t, []*v3high.Response{success}, found)
	})

	t.Run("accepts the 2xx wildcard", func(t *testing.T) {
		found := successResponses(responseFor(map[string]*v3high.Response{"2XX": success}, nil))
		assert.Equal(t, []*v3high.Response{success}, found)
	})

	t.Run("falls back to default when no success is documented", func(t *testing.T) {
		found := successResponses(responseFor(map[string]*v3high.Response{"500": failure}, fallback))
		assert.Equal(t, []*v3high.Response{fallback}, found)
	})

	t.Run("prefers an explicit success over default", func(t *testing.T) {
		found := successResponses(responseFor(map[string]*v3high.Response{"201": success}, fallback))
		assert.Equal(t, []*v3high.Response{success}, found)
	})

	t.Run("returns nothing when there is neither", func(t *testing.T) {
		assert.Empty(t, successResponses(responseFor(nil, nil)))
	})
}

func TestCollectStreamResponses(t *testing.T) {
	t.Run("is a no-op when streaming is disabled", func(t *testing.T) {
		options := streamOptions()
		options.ClientStreaming = false

		streams, types, err := collectStreamResponses("op", map[int]*orderedmap.Map[string, *v3high.MediaType]{
			200: mediaTypes("text/event-stream"),
		}, nil, options)

		require.NoError(t, err)
		assert.Empty(t, streams)
		assert.Empty(t, types)
	})

	t.Run("is a no-op without content", func(t *testing.T) {
		streams, types, err := collectStreamResponses("op", nil, nil, streamOptions())
		require.NoError(t, err)
		assert.Empty(t, streams)
		assert.Empty(t, types)
	})

	t.Run("skips statuses with no sequential media type", func(t *testing.T) {
		streams, _, err := collectStreamResponses("op", map[int]*orderedmap.Map[string, *v3high.MediaType]{
			200: mediaTypes("application/json"),
		}, nil, streamOptions())

		require.NoError(t, err)
		assert.Empty(t, streams)
	})

	t.Run("reports statuses in ascending order", func(t *testing.T) {
		streams, _, err := collectStreamResponses("op", map[int]*orderedmap.Map[string, *v3high.MediaType]{
			206: mediaTypes("application/x-ndjson"),
			200: mediaTypes("application/json", "text/event-stream"),
		}, nil, streamOptions())

		require.NoError(t, err)
		require.Len(t, streams, 2)
		assert.Equal(t, 200, streams[0].StatusCode)
		assert.Equal(t, "text/event-stream", streams[0].ContentType)
		assert.Equal(t, streamFramingSSE, streams[0].Framing)
		assert.Equal(t, 206, streams[1].StatusCode)
		assert.Equal(t, streamFramingLines, streams[1].Framing)

		// No schema at all, so frames are handed over verbatim.
		assert.Equal(t, streamRawItemType, streams[0].ItemName)
	})
}

func TestStreamItemType(t *testing.T) {
	model := loadStreamingModel(t, "streaming.yml")

	contentOf := func(t *testing.T, path, method, status, mediaType string) *v3high.MediaType {
		t.Helper()
		item := model.Paths.PathItems.Value(path)
		require.NotNil(t, item)
		op := item.GetOperations().Value(method)
		require.NotNil(t, op)
		response := op.Responses.Codes.Value(status)
		require.NotNil(t, response)
		content := response.Content.Value(mediaType)
		require.NotNil(t, content)
		return content
	}

	t.Run("resolves a $ref to the component type", func(t *testing.T) {
		content := contentOf(t, "/events", "get", "200", "text/event-stream")
		name, types, err := streamItemType(content, nil, GoSchema{}, "GetEvents", streamOptions())

		require.NoError(t, err)
		assert.Equal(t, "Event", name)
		assert.Empty(t, types, "a component is already generated")
	})

	t.Run("generates a named type for an inline schema", func(t *testing.T) {
		content := contentOf(t, "/logs", "get", "200", "application/x-ndjson")
		name, types, err := streamItemType(content, nil, GoSchema{}, "StreamLogs", streamOptions())

		require.NoError(t, err)
		assert.Equal(t, "StreamLogsResponseItem", name)
		require.Len(t, types, 1)
		assert.Equal(t, "StreamLogsResponseItem", types[0].Name)
		assert.Equal(t, SpecLocationResponse, types[0].SpecLocation)
	})

	t.Run("falls back to raw frames for a bare string schema", func(t *testing.T) {
		content := contentOf(t, "/raw", "get", "200", "text/event-stream")
		name, types, err := streamItemType(content, nil, GoSchema{}, "StreamRaw", streamOptions())

		require.NoError(t, err)
		assert.Equal(t, streamRawItemType, name)
		assert.Empty(t, types)
	})

	t.Run("reuses the schema the primary pass already generated", func(t *testing.T) {
		content := contentOf(t, "/logs", "get", "200", "application/x-ndjson")
		fallback := GoSchema{GoType: "AlreadyBuilt"}

		// bodySchema matches what streamItemType would pick, so the fallback is
		// used instead of walking the schema a second time.
		name, types, err := streamItemType(content, content.Schema, fallback, "StreamLogs", streamOptions())

		require.NoError(t, err)
		assert.Equal(t, "StreamLogsResponseItem", name)
		require.Len(t, types, 1)
		assert.Equal(t, "AlreadyBuilt", types[0].Schema.GoType)
	})

	t.Run("tolerates nil content", func(t *testing.T) {
		name, types, err := streamItemType(nil, nil, GoSchema{}, "op", streamOptions())
		require.NoError(t, err)
		assert.Equal(t, streamRawItemType, name)
		assert.Empty(t, types)
	})

	t.Run("returns raw frames when the media type has no schema", func(t *testing.T) {
		name, _, err := streamItemType(&v3high.MediaType{}, nil, GoSchema{}, "op", streamOptions())
		require.NoError(t, err)
		assert.Equal(t, streamRawItemType, name)
	})
}

func TestStreamItemTypeUsesItemSchema(t *testing.T) {
	model := loadStreamingModel(t, "streaming-item-schema.yml")

	contentOf := func(t *testing.T, path, method, mediaType string) *v3high.MediaType {
		t.Helper()
		op := model.Paths.PathItems.Value(path).GetOperations().Value(method)
		require.NotNil(t, op)
		return op.Responses.Codes.Value("200").Content.Value(mediaType)
	}

	t.Run("prefers itemSchema over schema", func(t *testing.T) {
		content := contentOf(t, "/chat", "post", "text/event-stream")
		require.NotNil(t, content.ItemSchema)
		require.Nil(t, content.Schema, "the fixture has no whole-body schema")

		name, types, err := streamItemType(content, nil, GoSchema{}, "Chat", streamOptions())
		require.NoError(t, err)
		assert.Equal(t, "Chunk", name)
		assert.Empty(t, types)
	})

	t.Run("generates an inline itemSchema once", func(t *testing.T) {
		content := contentOf(t, "/inline", "get", "application/jsonl")
		name, types, err := streamItemType(content, nil, GoSchema{}, "InlineItems", streamOptions())

		require.NoError(t, err)
		assert.Equal(t, "InlineItemsResponseItem", name)
		require.Len(t, types, 1)
	})
}

func TestStreamItemTypeFromRef(t *testing.T) {
	t.Run("honours a rename applied by the type tracker", func(t *testing.T) {
		options := streamOptions()
		options.typeTracker.registerRef("#/components/schemas/Event", "Event2")

		name, types, err := streamItemTypeFromRef("#/components/schemas/Event", options)
		require.NoError(t, err)
		assert.Equal(t, "Event2", name)
		assert.Empty(t, types)
	})

	t.Run("derives the name when the ref is unknown", func(t *testing.T) {
		name, _, err := streamItemTypeFromRef("#/components/schemas/Event", streamOptions())
		require.NoError(t, err)
		assert.Equal(t, "Event", name)
	})

	t.Run("reports an unusable ref", func(t *testing.T) {
		_, _, err := streamItemTypeFromRef("", streamOptions())
		require.Error(t, err)
	})
}

func TestRegisterStreamItemType(t *testing.T) {
	t.Run("disambiguates a colliding name", func(t *testing.T) {
		options := streamOptions()
		options.typeTracker.registerName("OpResponseItem")

		name, types, err := registerStreamItemType(GoSchema{GoType: "Frame"}, "Op", options)
		require.NoError(t, err)
		assert.NotEqual(t, "OpResponseItem", name)
		require.Len(t, types, 1)
		assert.Equal(t, name, types[0].Name)
	})

	t.Run("registers additional types only once", func(t *testing.T) {
		options := streamOptions()
		nested := TypeDefinition{Name: "Nested", Schema: GoSchema{GoType: "Nested"}}
		options.typeTracker.register(nested, "")

		_, types, err := registerStreamItemType(GoSchema{
			GoType:          "Frame",
			AdditionalTypes: []TypeDefinition{nested, {Name: "Fresh", Schema: GoSchema{GoType: "Fresh"}}},
		}, "Op", options)

		require.NoError(t, err)
		require.Len(t, types, 2, "the already-registered nested type is skipped")
		assert.Equal(t, "OpResponseItem", types[0].Name)
		assert.Equal(t, "Fresh", types[1].Name)
	})

	t.Run("falls back to raw frames for an undecodable schema", func(t *testing.T) {
		name, types, err := registerStreamItemType(GoSchema{GoType: "string"}, "Op", streamOptions())
		require.NoError(t, err)
		assert.Equal(t, streamRawItemType, name)
		assert.Empty(t, types)
	})
}

func TestWarnUnconsumableStreamsGuards(t *testing.T) {
	clientCfg := Configuration{Generate: &GenerateOptions{Client: true}}

	tests := []struct {
		name  string
		model *v3high.Document
		cfg   Configuration
	}{
		{name: "no generate options", model: &v3high.Document{}, cfg: Configuration{}},
		{name: "streaming already enabled", model: &v3high.Document{}, cfg: Configuration{Generate: &GenerateOptions{Client: true, ClientStreaming: true}}},
		{name: "no client generated", model: &v3high.Document{}, cfg: Configuration{Generate: &GenerateOptions{}}},
		{name: "nil model", cfg: clientCfg},
		{name: "model without paths", model: &v3high.Document{}, cfg: clientCfg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := captureLogs(t)
			warnUnconsumableStreams(tt.model, tt.cfg)
			assert.Empty(t, logs.String())
		})
	}
}

func TestRegisterStreamItemTypeArraySchema(t *testing.T) {
	options := streamOptions()

	name, types, err := registerStreamItemType(GoSchema{
		GoType:    "[]Frame",
		ArrayType: &GoSchema{GoType: "Frame"},
	}, "Op", options)

	require.NoError(t, err)
	assert.Equal(t, "OpResponseItem", name)
	require.Len(t, types, 1)
	assert.NotNil(t, types[0].Schema.ArrayType)
}

func TestWarnUnconsumableStreamsSkipsIncompletePaths(t *testing.T) {
	paths := orderedmap.New[string, *v3high.PathItem]()
	paths.Set("/nil-item", nil)
	paths.Set("/no-responses", &v3high.PathItem{Get: &v3high.Operation{}})

	noContent := &v3high.Responses{Codes: orderedmap.New[string, *v3high.Response]()}
	noContent.Codes.Set("200", &v3high.Response{})
	paths.Set("/no-content", &v3high.PathItem{Get: &v3high.Operation{Responses: noContent}})

	logs := captureLogs(t)
	warnUnconsumableStreams(
		&v3high.Document{Paths: &v3high.Paths{PathItems: paths}},
		Configuration{Generate: &GenerateOptions{Client: true}},
	)
	assert.Empty(t, logs.String())
}
