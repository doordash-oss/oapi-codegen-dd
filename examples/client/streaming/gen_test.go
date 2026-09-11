package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// httpClientAdapter adapts http.Client to runtime.HttpRequestDoer.
type httpClientAdapter struct {
	client *http.Client
}

func (a *httpClientAdapter) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	return a.client.Do(req)
}

// newClient points a generated client at srv.
func newClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	client, err := NewDefaultClient(srv.URL, runtime.WithHTTPClient(&httpClientAdapter{client: srv.Client()}))
	require.NoError(t, err)
	return client
}

// eventServer flushes count SSE frames, then whatever trailer is given.
func eventServer(t *testing.T, count int, trailer string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		// A comment line is a keep-alive and must not surface as a frame.
		_, _ = fmt.Fprint(w, ": open\n\n")
		for i := range count {
			_, _ = fmt.Fprintf(w,
				"id: %d\nevent: message\ndata: {\"seq\":%d,\"type\":\"created\",\"createdAt\":\"2026-09-11T09:00:0%dZ\",\"actor\":{\"id\":\"8b1b1a3e-0000-4000-8000-00000000000%d\",\"name\":\"actor-%d\"},\"tags\":[\"a\",\"b\"]}\n\n",
				i, i, i%10, i%10, i)
			flusher.Flush()
		}
		_, _ = fmt.Fprint(w, trailer)
		flusher.Flush()
	}))
}

func TestGetEvents_DecodesFramesInOrder(t *testing.T) {
	srv := eventServer(t, 3, "")
	defer srv.Close()

	stream, err := newClient(t, srv).GetEventsStream(context.Background())
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()

	var seqs []int
	for stream.Next() {
		// Frames decode into the full Event struct: enum, timestamp, nested
		// object and slice, not raw bytes.
		event := stream.Current()
		seqs = append(seqs, event.Seq)

		assert.Equal(t, Created, event.Type)
		assert.Equal(t, 2026, event.CreatedAt.Year())
		assert.Equal(t, fmt.Sprintf("actor-%d", event.Seq), event.Actor.Name)
		assert.NotEqual(t, uuid.Nil, event.Actor.ID)
		assert.Equal(t, []string{"a", "b"}, event.Tags)

		assert.Equal(t, "message", stream.Event().Type)
	}

	require.NoError(t, stream.Err())
	assert.Equal(t, []int{0, 1, 2}, seqs)
}

func TestGetEvents_AllStopsOnBreak(t *testing.T) {
	srv := eventServer(t, 100, "")
	defer srv.Close()

	stream, err := newClient(t, srv).GetEventsStream(context.Background())
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()

	seen := 0
	for _, err := range stream.All() {
		require.NoError(t, err)
		seen++
		if seen == 2 {
			break
		}
	}

	assert.Equal(t, 2, seen)
	assert.NoError(t, stream.Close())
}

func TestGetEvents_SentinelEndsStream(t *testing.T) {
	srv := eventServer(t, 2, "data: [DONE]\n\n")
	defer srv.Close()

	stream, err := newClient(t, srv).GetEventsStream(context.Background())
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()

	// Without this, "[DONE]" would surface as a JSON decode error.
	stream.Sentinels = []string{"[DONE]"}

	seen := 0
	for stream.Next() {
		seen++
	}

	require.NoError(t, stream.Err())
	assert.Equal(t, 2, seen)
}

func TestGetEvents_ContextCancellationAborts(t *testing.T) {
	srv := eventServer(t, 1000, "")
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := newClient(t, srv).GetEventsStream(ctx)
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()

	require.True(t, stream.Next())
	cancel()

	for stream.Next() {
		// Drain whatever was already buffered.
	}
	assert.ErrorIs(t, stream.Err(), context.Canceled)
}

func TestGetEvents_ErrorStatusStillDecodesJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"message":"upstream unavailable"}`)
	}))
	defer srv.Close()

	client := newClient(t, srv)

	stream, err := client.GetEventsStream(context.Background())
	require.Error(t, err)
	assert.Nil(t, stream)

	// The streaming sibling reports the status the same way the non-streaming
	// method does.
	var apiErr *runtime.ClientAPIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode())

	// The envelope form carries the decoded error body.
	resp, err := client.GetEventsStreamWithResponse(context.Background())
	require.Error(t, err)
	assert.Nil(t, resp.Stream200)
	require.NotNil(t, resp.JSON500)
	assert.Equal(t, "upstream unavailable", resp.JSON500.Message)
}

// chatServer answers /chat with a single JSON document, or an SSE stream when
// the request body asks for one - the shape most LLM APIs use.
func chatServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Prompt string `json:"prompt"`
			Stream bool   `json:"stream"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		if !body.Stream {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"id":"cmpl-1","text":%q,"usage":{"promptTokens":3,"completionTokens":7}}`, "hello "+body.Prompt)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)
		_, _ = fmt.Fprint(w, "data: {\"index\":0,\"delta\":\"he\"}\n\n")
		flusher.Flush()
		_, _ = fmt.Fprint(w, "data: {\"index\":1,\"delta\":\"llo\",\"finishReason\":\"stop\"}\n\n")
		flusher.Flush()
	}))
}

// One operation declaring both application/json and text/event-stream exposes
// both shapes: the generator never has to pick a winner.
func TestChat_BothShapesFromOneOperation(t *testing.T) {
	srv := chatServer(t)
	defer srv.Close()
	client := newClient(t, srv)

	completion, err := client.Chat(context.Background(), &ChatRequestOptions{
		Body: &ChatBody{Prompt: "world"},
	})
	require.NoError(t, err)
	assert.Equal(t, "hello world", completion.Text)
	assert.Equal(t, 7, completion.Usage.CompletionTokens)

	stream, err := client.ChatStream(context.Background(), &ChatRequestOptions{
		Body: &ChatBody{Prompt: "world", Stream: runtime.Ptr(true)},
	})
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()

	var (
		deltas []string
		last   Chunk
	)
	for chunk, err := range stream.All() {
		require.NoError(t, err)
		deltas = append(deltas, chunk.Delta)
		last = chunk
	}
	assert.Equal(t, []string{"he", "llo"}, deltas)
	assert.Equal(t, 1, last.Index)
	require.NotNil(t, last.FinishReason)
	assert.Equal(t, Stop, *last.FinishReason)
}

// The same envelope type carries either shape, populated by whichever method
// was called.
func TestChat_EnvelopeCarriesEitherShape(t *testing.T) {
	srv := chatServer(t)
	defer srv.Close()
	client := newClient(t, srv)

	jsonResp, err := client.ChatWithResponse(context.Background(), &ChatRequestOptions{
		Body: &ChatBody{Prompt: "world"},
	})
	require.NoError(t, err)
	require.NotNil(t, jsonResp.JSON200)
	assert.Equal(t, "hello world", jsonResp.JSON200.Text)
	assert.Nil(t, jsonResp.Stream200)

	streamResp, err := client.ChatStreamWithResponse(context.Background(), &ChatRequestOptions{
		Body: &ChatBody{Prompt: "world", Stream: runtime.Ptr(true)},
	})
	require.NoError(t, err)
	require.NotNil(t, streamResp.Stream200)
	defer func() { _ = streamResp.Stream200.Close() }()
	assert.Nil(t, streamResp.JSON200)
	assert.True(t, streamResp.Stream200.Next())
}

// Asking for a stream from a server that answers with JSON is reported, rather
// than handed back as a stream that silently yields nothing.
func TestChatStream_ServerDidNotStream(t *testing.T) {
	srv := chatServer(t)
	defer srv.Close()

	// stream is omitted, so the server replies with a JSON document.
	stream, err := newClient(t, srv).ChatStream(context.Background(), &ChatRequestOptions{
		Body: &ChatBody{Prompt: "world"},
	})
	require.Error(t, err)
	assert.Nil(t, stream)
	assert.Contains(t, err.Error(), "expected a text/event-stream stream")
}

func TestStreamLogs_DecodesLineDelimitedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/x-ndjson", r.Header.Get("Accept"))

		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = fmt.Fprint(w,
			"{\"level\":\"info\",\"message\":\"first\",\"at\":\"2026-09-11T09:00:00Z\",\"source\":{\"service\":\"api\",\"host\":\"a1\"}}\n"+
				"\n"+
				"{\"level\":\"warn\",\"message\":\"second\",\"at\":\"2026-09-11T09:00:01Z\"}\n")
	}))
	defer srv.Close()

	stream, err := newClient(t, srv).StreamLogsStream(context.Background())
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()

	var (
		messages []string
		levels   []StreamLogsResponseLevel
	)
	for record, err := range stream.All() {
		require.NoError(t, err)
		messages = append(messages, record.Message)
		levels = append(levels, record.Level)
		assert.Equal(t, 2026, record.At.Year())
	}

	assert.Equal(t, []string{"first", "second"}, messages)
	assert.Equal(t, []StreamLogsResponseLevel{Info, Warn}, levels)
}

func TestGetEventsWithResponse_ExposesStreamOnEnvelope(t *testing.T) {
	srv := eventServer(t, 2, "")
	defer srv.Close()

	resp, err := newClient(t, srv).GetEventsStreamWithResponse(context.Background())
	require.NoError(t, err)
	require.NotNil(t, resp.Stream200)
	defer func() { _ = resp.Stream200.Close() }()

	// A streaming status is never buffered, so Body stays empty.
	assert.Empty(t, resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	seen := 0
	for resp.Stream200.Next() {
		seen++
	}
	require.NoError(t, resp.Stream200.Err())
	assert.Equal(t, 2, seen)
}
