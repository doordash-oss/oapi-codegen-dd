// Copyright 2025 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package runtime

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type streamEvent struct {
	Seq int    `json:"seq"`
	Msg string `json:"msg"`
}

// trackingBody counts Close calls and can fail on Close.
type trackingBody struct {
	io.Reader
	closes   int
	closeErr error
}

func (b *trackingBody) Close() error {
	b.closes++
	return b.closeErr
}

// errReader always fails, standing in for a broken connection.
type errReader struct {
	err error
}

func (r *errReader) Read([]byte) (int, error) {
	return 0, r.err
}

func streamResponse(contentType, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestIsSequentialMediaType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		expected    bool
	}{
		{name: "server-sent events", contentType: "text/event-stream", expected: true},
		{name: "server-sent events with charset", contentType: "text/event-stream; charset=utf-8", expected: true},
		{name: "ndjson", contentType: "application/x-ndjson", expected: true},
		{name: "ndjson without prefix", contentType: "application/ndjson", expected: true},
		{name: "jsonl", contentType: "application/jsonl", expected: true},
		{name: "jsonlines", contentType: "application/x-jsonlines", expected: true},
		{name: "json-lines", contentType: "application/json-lines", expected: true},
		{name: "uppercase is normalized", contentType: "TEXT/EVENT-STREAM", expected: true},
		{name: "unparseable falls back to the prefix", contentType: "text/event-stream; charset", expected: true},
		{name: "plain json is not sequential", contentType: "application/json", expected: false},
		{name: "empty", contentType: "", expected: false},
		{name: "json sequences are not supported", contentType: "application/json-seq", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsSequentialMediaType(tt.contentType))
		})
	}
}

func TestStream_EventFraming(t *testing.T) {
	body := ": keep-alive\n" +
		"id: 1\nevent: message\nretry: 250\ndata: {\"seq\":1,\"msg\":\"first\"}\n\n" +
		"\n" + // a blank line with no payload is a separator, not a frame
		"event: ping\n\n" + // a frame without data is not dispatched
		"data: {\"seq\":2,\n" +
		"data:  \"msg\":\"second\"}\n\n" +
		"data: {\"seq\":3,\"msg\":\"third\"}" // no trailing blank line

	stream := NewEventStream[streamEvent](streamResponse("text/event-stream", body))
	defer func() { assert.NoError(t, stream.Close()) }()

	require.True(t, stream.Next())
	assert.Equal(t, streamEvent{Seq: 1, Msg: "first"}, stream.Current())
	assert.Equal(t, "1", stream.Event().ID)
	assert.Equal(t, "message", stream.Event().Type)
	assert.Equal(t, 250*time.Millisecond, stream.Event().Retry)

	require.True(t, stream.Next())
	assert.Equal(t, streamEvent{Seq: 2, Msg: "second"}, stream.Current())
	// A second data line keeps only the field value, so the space survives.
	assert.Equal(t, "{\"seq\":2,\n \"msg\":\"second\"}", string(stream.Event().Data))

	require.True(t, stream.Next())
	assert.Equal(t, streamEvent{Seq: 3, Msg: "third"}, stream.Current())

	assert.False(t, stream.Next())
	assert.NoError(t, stream.Err())
	// Calling Next past the end stays false.
	assert.False(t, stream.Next())
}

func TestStream_EventFramingIgnoresInvalidFields(t *testing.T) {
	body := "id: bad\x00id\nretry: soon\ndata: {\"seq\":1}\n\n" +
		"retry: -5\nunknown: x\nnocolon\ndata: {\"seq\":2}\n\n"

	stream := NewEventStream[streamEvent](streamResponse("text/event-stream", body))
	defer func() { _ = stream.Close() }()

	require.True(t, stream.Next())
	assert.Empty(t, stream.Event().ID)
	assert.Zero(t, stream.Event().Retry)

	require.True(t, stream.Next())
	assert.Zero(t, stream.Event().Retry)
	assert.False(t, stream.Next())
	assert.NoError(t, stream.Err())
}

func TestStream_LineTerminators(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "line feed", body: "data: {\"seq\":1}\n\ndata: {\"seq\":2}\n\n"},
		{name: "carriage return line feed", body: "data: {\"seq\":1}\r\n\r\ndata: {\"seq\":2}\r\n\r\n"},
		{name: "carriage return", body: "data: {\"seq\":1}\r\rdata: {\"seq\":2}\r\r"},
		{name: "byte order mark", body: "\ufeffdata: {\"seq\":1}\n\ndata: {\"seq\":2}\n\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := NewEventStream[streamEvent](streamResponse("text/event-stream", tt.body))
			defer func() { _ = stream.Close() }()

			var seqs []int
			for stream.Next() {
				seqs = append(seqs, stream.Current().Seq)
			}
			require.NoError(t, stream.Err())
			assert.Equal(t, []int{1, 2}, seqs)
		})
	}
}

func TestStream_LineFraming(t *testing.T) {
	body := "{\"seq\":1}\n\n  \n{\"seq\":2}\n{\"seq\":3}"

	stream := NewLineStream[streamEvent](streamResponse("application/x-ndjson", body))
	defer func() { _ = stream.Close() }()

	var seqs []int
	for stream.Next() {
		seqs = append(seqs, stream.Current().Seq)
		assert.NotEmpty(t, stream.Event().Data)
		assert.Empty(t, stream.Event().Type)
	}

	require.NoError(t, stream.Err())
	assert.Equal(t, []int{1, 2, 3}, seqs)
}

func TestNewStream_PicksFramingFromContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
	}{
		{name: "event stream", contentType: "text/event-stream", body: "data: {\"seq\":7}\n\n"},
		{name: "line delimited", contentType: "application/jsonl", body: "{\"seq\":7}\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := NewStream[streamEvent](streamResponse(tt.contentType, tt.body))
			defer func() { _ = stream.Close() }()

			require.True(t, stream.Next())
			assert.Equal(t, 7, stream.Current().Seq)
		})
	}
}

func TestNewStream_MissingBody(t *testing.T) {
	tests := []struct {
		name string
		resp *http.Response
	}{
		{name: "nil response", resp: nil},
		{name: "nil body", resp: &http.Response{StatusCode: http.StatusOK}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := NewStream[streamEvent](tt.resp)
			assert.False(t, stream.Next())
			assert.NoError(t, stream.Err())
			assert.NoError(t, stream.Close())
		})
	}
}

func TestStream_ItemTypes(t *testing.T) {
	t.Run("bytes are copied verbatim", func(t *testing.T) {
		stream := NewEventStream[[]byte](streamResponse("text/event-stream", "data: not json\n\n"))
		defer func() { _ = stream.Close() }()

		require.True(t, stream.Next())
		assert.Equal(t, []byte("not json"), stream.Current())
	})

	t.Run("strings are copied verbatim", func(t *testing.T) {
		stream := NewEventStream[string](streamResponse("text/event-stream", "data: not json\n\n"))
		defer func() { _ = stream.Close() }()

		require.True(t, stream.Next())
		assert.Equal(t, "not json", stream.Current())
	})

	t.Run("raw messages are copied verbatim", func(t *testing.T) {
		stream := NewEventStream[json.RawMessage](streamResponse("text/event-stream", "data: {\"seq\":1}\n\n"))
		defer func() { _ = stream.Close() }()

		require.True(t, stream.Next())
		assert.JSONEq(t, `{"seq":1}`, string(stream.Current()))
	})

	t.Run("anything else is unmarshalled", func(t *testing.T) {
		stream := NewEventStream[map[string]int](streamResponse("text/event-stream", "data: {\"seq\":1}\n\n"))
		defer func() { _ = stream.Close() }()

		require.True(t, stream.Next())
		assert.Equal(t, map[string]int{"seq": 1}, stream.Current())
	})
}

func TestStream_DecodeError(t *testing.T) {
	stream := NewEventStream[streamEvent](streamResponse("text/event-stream", "data: [DONE]\n\ndata: {\"seq\":1}\n\n"))
	defer func() { _ = stream.Close() }()

	assert.False(t, stream.Next())
	require.Error(t, stream.Err())
	assert.Contains(t, stream.Err().Error(), "error decoding stream frame")
}

func TestStream_Sentinels(t *testing.T) {
	body := "data: {\"seq\":1}\n\ndata: [DONE]\n\ndata: {\"seq\":2}\n\n"

	stream := NewEventStream[streamEvent](streamResponse("text/event-stream", body))
	defer func() { _ = stream.Close() }()
	stream.Sentinels = []string{"[DONE]"}

	require.True(t, stream.Next())
	assert.Equal(t, 1, stream.Current().Seq)

	// The sentinel ends the stream cleanly, and anything after it is ignored.
	assert.False(t, stream.Next())
	assert.NoError(t, stream.Err())
}

func TestStream_ReadError(t *testing.T) {
	readErr := errors.New("connection reset")

	t.Run("event framing", func(t *testing.T) {
		body := io.MultiReader(strings.NewReader("data: {\"seq\":1}\n\ndata: partial\n"), &errReader{err: readErr})
		stream := NewEventStream[streamEvent](&http.Response{Body: io.NopCloser(body)})
		defer func() { _ = stream.Close() }()

		require.True(t, stream.Next())
		assert.False(t, stream.Next())
		assert.ErrorIs(t, stream.Err(), readErr)
	})

	t.Run("line framing", func(t *testing.T) {
		body := io.MultiReader(strings.NewReader("{\"seq\":1}\n"), &errReader{err: readErr})
		stream := NewLineStream[streamEvent](&http.Response{Body: io.NopCloser(body)})
		defer func() { _ = stream.Close() }()

		require.True(t, stream.Next())
		assert.False(t, stream.Next())
		assert.ErrorIs(t, stream.Err(), readErr)
	})

	t.Run("only the first error is kept", func(t *testing.T) {
		stream := NewLineStream[streamEvent](&http.Response{Body: io.NopCloser(&errReader{err: readErr})})
		defer func() { _ = stream.Close() }()

		assert.False(t, stream.Next())
		stream.fail(errors.New("later"))
		assert.ErrorIs(t, stream.Err(), readErr)
	})
}

func TestStream_Close(t *testing.T) {
	t.Run("is idempotent", func(t *testing.T) {
		body := &trackingBody{Reader: strings.NewReader("data: {\"seq\":1}\n\n")}
		stream := NewEventStream[streamEvent](&http.Response{Body: body})

		require.NoError(t, stream.Close())
		require.NoError(t, stream.Close())
		assert.Equal(t, 1, body.closes)
		// A closed stream yields nothing more.
		assert.False(t, stream.Next())
	})

	t.Run("reports the underlying error", func(t *testing.T) {
		closeErr := errors.New("already gone")
		body := &trackingBody{Reader: strings.NewReader(""), closeErr: closeErr}
		stream := NewEventStream[streamEvent](&http.Response{Body: body})

		assert.ErrorIs(t, stream.Close(), closeErr)
	})
}

func TestStream_All(t *testing.T) {
	t.Run("yields every frame", func(t *testing.T) {
		stream := NewLineStream[streamEvent](streamResponse("application/jsonl", "{\"seq\":1}\n{\"seq\":2}\n"))
		defer func() { _ = stream.Close() }()

		var seqs []int
		for event, err := range stream.All() {
			require.NoError(t, err)
			seqs = append(seqs, event.Seq)
		}
		assert.Equal(t, []int{1, 2}, seqs)
	})

	t.Run("stops when the caller breaks", func(t *testing.T) {
		body := &trackingBody{Reader: strings.NewReader("{\"seq\":1}\n{\"seq\":2}\n{\"seq\":3}\n")}
		stream := NewLineStream[streamEvent](&http.Response{Body: body})

		seen := 0
		for range stream.All() {
			seen++
			break
		}

		assert.Equal(t, 1, seen)
		require.NoError(t, stream.Close())
		assert.Equal(t, 1, body.closes)
	})

	t.Run("yields a terminal error", func(t *testing.T) {
		stream := NewLineStream[streamEvent](streamResponse("application/jsonl", "{\"seq\":1}\nnot json\n"))
		defer func() { _ = stream.Close() }()

		var lastErr error
		seen := 0
		for _, err := range stream.All() {
			if err != nil {
				lastErr = err
				continue
			}
			seen++
		}

		assert.Equal(t, 1, seen)
		require.Error(t, lastErr)
		assert.Contains(t, lastErr.Error(), "error decoding stream frame")
	})
}
