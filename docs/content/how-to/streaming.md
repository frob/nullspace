---
title: Stream large lists as NDJSON
weight: 7
---

Use this guide when a list endpoint returns enough rows that buffering them
all into memory is wasteful.

## Solution

Add `?stream=true` to a list request. The response switches to
`application/x-ndjson` with one JSON object per line.

```
GET /api/posts?stream=true

{"ID":"hello","Body":"...","title":"Hello World"}
{"ID":"goodbye","Body":"...","title":"Goodbye"}
```

Streaming works on any route that uses the built-in `data.list` handler.
No code changes are required.

## Variations

### Always stream a specific route

Set `stream = "true"` in the route's `extra` table. The handler then ignores
the query parameter:

```toml
[[routing.routes]]
path       = "/api/posts"
handler    = "data.list"
collection = "posts"
extra      = { stream = "true" }
```

### Stream from a custom handler

`WriteStream` takes a `StreamIter` that yields items one at a time:

```go
sr := &response.StreamResponse{
    Status: http.StatusOK,
    Meta:   map[string]any{"Collection": "posts"},
    Iter:   myIter,
}
return pipeline.WriteStream(ctx.Context(), ctx.Writer, sr)
```

```go
type myIter struct{ /* ... */ }

func (it *myIter) Next() (any, error) {
    // return next item, or (nil, nil) when done
}
func (it *myIter) Close() error { return nil }
```

### Stream over TCP/IPC

The `data.bridge` module accepts `"stream": true` in the payload and replies
with `data.list.start`, `data.list.item`, and `data.list.end` envelopes.
See [Expose data over TCP/IPC]({{< relref "data-bridge" >}}).

## How format selection interacts with streaming

Streaming uses a `StreamFormatter`. The built-in JSON formatter implements
it (NDJSON output). If a route forces a non-streamable format, streaming is
ignored and the request falls back to the buffered list response.

## See also

- [Generate CRUD routes for a collection]({{< relref "collections" >}})
- [Expose data over TCP/IPC]({{< relref "data-bridge" >}})
