# Streaming Responses

Some responses are not a single document but an open-ended sequence of items: a
Server-Sent Events feed, or a newline-delimited JSON log tail. Buffering one of
those blocks until the server closes the connection, which for an endless stream
never happens.

With [`generate.client-streaming`](configuration.md#generateclient-streaming),
every operation that documents a sequential response gains a `<Op>Stream`
sibling returning a live `runtime.Stream[T]` over the per-frame type.

## Overview

A response is treated as sequential when its media type is one of:

| Media type | Framing |
|---|---|
| `text/event-stream` | Server-Sent Events |
| `application/x-ndjson`, `application/ndjson` | one JSON value per line |
| `application/jsonl`, `application/x-jsonlines`, `application/json-lines` | one JSON value per line |

The per-frame type comes from:

1. `itemSchema` (OpenAPI 3.2), which describes exactly one item, or
2. `schema`, which is how 3.0 and 3.1 specs conventionally describe an event payload.

A `$ref` reuses the component type. An inline object becomes
`<OperationID>ResponseItem`. When there is no schema that can be JSON-decoded
(none at all, or a bare `string`), frames are handed over as raw `[]byte`.

**The non-streaming methods are never changed.** An operation declaring both
`application/json` and `text/event-stream` at one status exposes both shapes, so
the generator never has to pick a winner and you never need an overlay to get at
the one you want.

## Example

```yaml title="examples/client/streaming/api.yaml"
--8<-- "client/streaming/api.yaml:6:31"
```

```yaml title="examples/client/streaming/cfg.yaml"
--8<-- "client/streaming/cfg.yaml"
```

## Generated Code

```go
--8<-- "client/streaming/gen.go:38:42"
```

`Chat` returns the JSON document; `ChatStream` returns a live stream of typed
frames - `Chunk` here, generated from the media type's schema like any other
response body:

```go
--8<-- "client/streaming/gen.go:778:782"
```

With `generate.client-with-response` on, one envelope type carries either shape,
populated by whichever method was called:

```go
--8<-- "client/streaming/gen.go:725:731"
```

## Consuming a Stream

`runtime.Stream[T]` is `bufio.Scanner`-shaped. **The caller owns the
connection and must close the stream.**

```go
stream, err := client.GetEventsStream(ctx)
if err != nil {
    return err
}
defer stream.Close()

for stream.Next() {
    event := stream.Current()  // Event: enum, time.Time, nested struct, slice
    fmt.Println(event.Seq, event.Type, event.Actor.Name)
}
return stream.Err()
```

`All()` gives the same loop as a range-over-func iterator, with errors
delivered inline:

```go
for event, err := range stream.All() {
    if err != nil {
        return err
    }
    fmt.Println(event.Seq, event.CreatedAt)
}
```

`Event()` returns the raw frame behind `Current()` - the Server-Sent Events
`id`, `event` and `retry` fields. For line-delimited JSON only `Data` is set.

### Stopping Early

Either break out of the loop and `Close()`, or cancel the context the request
was made with. Cancelling unblocks the pending read and `Err()` reports
`context.Canceled`.

### Terminators That Are Not JSON

OpenAI-compatible APIs end a stream with `data: [DONE]`, which is not valid
JSON and would otherwise surface as a decode error. Set `Sentinels` before the
first `Next()`:

```go
stream.Sentinels = []string{"[DONE]"}
```

### Asking the Server to Stream

For the common pattern where one endpoint answers either way, the server
decides based on the request, not on the method you called. `<Op>Stream` sends
`Accept: <media type>`, but it cannot know which request field toggles
streaming - you still have to set it:

```go
stream, err := client.ChatStream(ctx, &ChatRequestOptions{
    Body: &ChatBody{Prompt: "hello", Stream: runtime.Ptr(true)},
})
```

If the server answers with a single JSON document anyway, `<Op>Stream` returns
an error naming the Content-Type it got, rather than handing back a stream that
silently yields nothing.

## How Buffering Is Skipped

`runtime.Client.ExecuteRequest` normally reads the whole body into
`Response.Content`. It leaves the body unread only when all three of the
following hold:

1. the request was marked as streaming - the generated sibling sets
   `RequestOptionsParameters.Stream` to the media type, which also supplies the
   `Accept` header when the caller has not set one;
2. the response `Content-Type` is a sequential media type, so a server that
   ignored `Accept` and replied with JSON is still buffered; and
3. the status is 2xx, so error responses always reach the usual decode path and
   `<Op>Stream` can report a documented error body just like `<Op>` does.

`Response.Streaming` reports which happened. When it is true, `Content` is nil
and `Raw.Body` is still open, which is why the envelope's `Body` field is empty
for a streamed status.

A custom `runtime.APIClient` implementation can honour the same marker with
`runtime.IsStreamingResponse(ctx)`, and set it with
`runtime.WithStreamingResponse(ctx)`.

## Consuming a Stream Without Codegen

The stream helpers work off a plain `*http.Response`, so they are usable
anywhere - off the envelope's `HTTPResponse`, or from a hand-written doer:

```go
stream := runtime.NewStream[MyEvent](httpResp) // framing from Content-Type
defer stream.Close()
```

`NewEventStream` and `NewLineStream` pick the framing explicitly.

## Limitations

- **Client-side only.** Handler and server generation is unaffected: the
  response type keeps its `[]byte` shape, so a generated server writes a
  sequential response as a pre-marshaled body. Writing Server-Sent Events from
  a generated handler is not supported yet.
- **A non-streaming method on a streaming-only response still blocks.** If the
  only media type documented at a status is sequential, `<Op>` buffers it and
  blocks forever, exactly as it did before this feature existed. Use `<Op>Stream`.
  Generation warns about these operations by name when the flag is off, so you
  do not have to discover it at runtime.
- **Request bodies are not streamed.**
- `multipart/mixed` and `application/json-seq` (RFC 7464) framing are not
  supported. `application/stream+json` is treated as plain JSON.
- **MCP tools.** A sequential response cannot be serialised into a single tool
  result, so the generated MCP tool for such an operation returns an error.

## Full Example

[View the complete example](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/client/streaming/){:target="_blank"}
