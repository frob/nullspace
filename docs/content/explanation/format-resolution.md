---
title: Format resolution
weight: 6
---

A handler in Nullspace returns a `*Response` value, not bytes. The
response has a status code, optional headers, and a `Data` payload —
some struct, map, or slice. The framework picks the wire format
afterwards. The same handler can produce JSON, HTML, plain text, ANSI
coloured text, or NDJSON, and the handler does not know which.

This page explains why this is the design, how the format actually gets
chosen, and what streaming responses do that buffered ones cannot.

## Why decouple handlers from formats

The conventional approach is the opposite: the handler writes the
response. `c.JSON(200, data)` returns JSON. `c.HTML(200, "tpl", data)`
renders a template. The handler decides.

This works well when a route only ever produces one format. It works
badly when:

- The same data is wanted as JSON for an API client, HTML for a
  browser, and plain text for a `curl` user.
- A TUI client wants ANSI-coloured output.
- A long list needs to be streamed as NDJSON for one client and rendered
  whole as JSON for another.

The handler-decides model forces you to write a separate handler per
format, or to put the format dispatch inside the handler, or to put the
format in the URL (`/posts.json`, `/posts.html`). Each option has
downsides.

Nullspace's split — handler produces `Data`, framework chooses format —
trades a slightly more abstract handler signature for the property that
*one handler serves every format*. Adding a new format adds a formatter
module; it does not touch any handler.

## The resolution chain

Format selection is a [resolution hook]({{< relref
"/explanation/hooks" >}}) at the point `response.format.resolve`. The
hook bus runs resolvers in priority order; the first to return
`(value, true, nil)` wins.

Four resolvers ship with the framework:

| Priority | Module                       | Source                        |
|----------|------------------------------|-------------------------------|
| 10       | `format.route_override`      | Route metadata in the TOML    |
| 20       | `format.query_param`         | `?format=json` (or similar)   |
| 30       | `format.content_negotiate`   | `Accept` request header       |
| 40       | `format.default`             | Configured default            |

Each is a separate module that can be independently enabled or
disabled. Disabling `format.query_param` means clients cannot override
the format with `?format=`; the chain falls through to the next resolver.
Disabling `format.default` means a request that does not match any
earlier resolver fails to resolve and gets the framework's last-resort
behaviour.

The ordering encodes a deliberate precedence:

1. **Route override wins.** If the route's TOML metadata says
   `format = "json"`, that is the format. The route author has made an
   explicit choice and clients cannot override it. This is right for
   API endpoints that must always be JSON.
2. **Query parameter beats Accept.** A user with `?format=html` overrides
   their browser's `Accept` header. This is right for debugging — you
   can hit an API endpoint with `?format=text` in a terminal and read
   the response without configuring curl.
3. **Accept header beats default.** A client that asks for
   `Accept: application/json` gets JSON. A browser asking for
   `text/html` gets HTML. Standard HTTP content negotiation.
4. **Default catches the rest.** Requests with no preference get the
   configured default (`response.default_format` in the TOML).

The reason this is a resolution chain and not an `if/else` ladder is
that each step is its own module. You can disable any step. You can add
a fifth resolver — say, "look at the file extension in the URL" — by
writing a module that registers at priority 15. The order is the
priority numbers; the policy is which modules you enable.

## Formatters

A formatter takes the resolved format name and the response value, and
produces bytes:

```go
type Formatter interface {
    Name() string
    ContentType() string
    Format(ctx *Context, resp *Response) ([]byte, error)
}
```

Built-in formatters:

| Format | Content-Type             | What it does                              |
|--------|--------------------------|-------------------------------------------|
| `json` | `application/json`       | `encoding/json` round trip                |
| `html` | `text/html`              | `html/template` render of `Response.Template` with `Data` |
| `text` | `text/plain`             | Structured key-value text, body afterwards|
| `ansi` | `text/plain`             | Same as `text` plus ANSI escape codes     |

The `text` and `ansi` formatters exist for terminal use. `text` is what
you get when you `curl` an endpoint without specifying a content type —
it lays out the response as `key: value` lines, a blank line, then the
body. `ansi` is the same with colour codes added, intended for TUI
clients and developers reading responses in their terminal.

The HTML formatter reads `Response.Template` to choose a template file
and passes `Data` as the template context. This is the bridge from the
format-agnostic handler model to the format-specific concern of
templates. The handler still does not pick HTML; it always sets
`Template`, and the formatter only uses it if HTML was resolved.

## What a handler looks like

```go
func listPosts(ctx *request.Context) error {
    posts := getPosts()
    resp := response.NewResponse(http.StatusOK, posts)
    resp.Template = "posts.html"  // used only if HTML is resolved
    return pipeline.Write(ctx.Context(), ctx.Writer, resp)
}
```

The handler does not branch on format. It produces a value, names a
template (if HTML is a possible outcome), and hands the response to the
pipeline. The pipeline runs format resolution, looks up the formatter,
fires `response.before_write`, serialises, writes, and fires
`response.after_write`.

This is the property that lets one handler back the same data over JSON,
HTML, plain text, and ANSI without any conditional code.

## Buffered versus streaming

The default pipeline is buffered: the formatter receives the entire
`Response`, produces a `[]byte`, and the pipeline writes it. For most
responses this is fine and simpler.

Streaming is opt-in. The framework supports two streaming formats out of
the box: NDJSON over HTTP and envelope messages over TCP/IPC.

### NDJSON over HTTP

```
GET /api/posts?stream=true
```

The response is `application/x-ndjson` — one JSON object per line:

```
{"ID":"hello","Body":"...","title":"Hello World"}
{"ID":"goodbye","Body":"...","title":"Goodbye"}
```

This lets the client process items as they arrive rather than waiting
for the full collection. It also avoids holding the entire collection in
memory on the server.

Streaming can be requested per-request with `?stream=true` or set
per-route in the TOML:

```toml
[[routing.routes]]
path = "/api/posts"
handler = "data.list"
collection = "posts"
extra = { stream = "true" }
```

### Envelope messages over TCP/IPC

The TCP and IPC transports use envelope messages to bracket a streamed
result:

```json
{"command":"data.list.start","payload":{"collection":"posts","total":2}}
{"command":"data.list.item","payload":{"ID":"hello",...}}
{"command":"data.list.item","payload":{"ID":"goodbye",...}}
{"command":"data.list.end","payload":{"collection":"posts","count":2}}
```

`.start` and `.end` give the client a clean boundary; each `.item`
carries one entity. This matches the line-oriented nature of the TCP
codec (each frame is one message) and works the same way on Unix
sockets.

### What streaming costs

Streaming is not free. The handler has to produce an iterator over the
data — for the file backend, this means a lazy iterator over the
filesystem rather than a slice; for SQL, a `Rows` iterator. The
response pipeline holds the writer open for the duration of the
iteration. Connection limits apply more aggressively because individual
streams can last for seconds. Buffered responses are usually right; you
opt in to streaming when buffered would be wrong.

The general guidance is the same as for any web server: stream when the
collection size could be unbounded or large enough that holding it in
memory is a problem; buffer otherwise.

## What was given up

The format-resolution model has costs worth naming.

**Indirection.** A reader following the path of a request has more
boxes to traverse than in a framework where the handler writes JSON
directly. You have to know about the resolution chain, the formatter
registry, and the `Response` value. The first time it is
unfamiliar.

**Coercion errors push later.** If `Data` contains something that JSON
can serialise but HTML cannot, the failure happens in the formatter,
not in the handler. The error path needs to be good. The framework
fires `request.error` and serialises a default error response in the
resolved format, but you still need to think about it.

**Streaming changes the contract.** A streaming handler cannot signal a
404 by returning an `http.StatusNotFound`, because by the time the
404 is known, the stream is already open with `200 OK`. Streaming
handlers have to check existence before they start streaming.

The benefit is that one handler produces every representation. For a
content-heavy application — exactly the kind Nullspace is built for —
that is the right tradeoff.

## Where to read next

- [The hook bus]({{< relref "/explanation/hooks" >}}) — resolution
  hooks in general.
- [Multi-transport]({{< relref "/explanation/transports" >}}) — how
  streaming differs between HTTP, TCP, and IPC.
- [Format resolvers reference]({{< relref "/reference" >}}) — every
  resolver, its priority, and its TOML key.
