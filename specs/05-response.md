# Response Specification

The response layer serializes handler output into the requested format using a pipeline of formatter modules.

## Responsibilities

- Resolve the response format via the format resolution chain
- Serialize data using the appropriate formatter
- Write the HTTP response with correct headers

## Response Value

Handlers return a response value, not raw bytes:

```go
type Response struct {
    Status  int
    Headers map[string]string
    Data    interface{}   // the payload — struct, map, slice, etc.
    Template string       // optional template name for HTML rendering
    Error   error         // if non-nil, triggers error handling
}
```

The response layer is responsible for serializing `Data` into the resolved format.

## Format Resolution

Format is resolved through a prioritized, short-circuit hook chain at `response.format.resolve`. Each resolver is its own module:

```
Module: FormatRouteOverride     -> priority 10 (checks route metadata)
Module: FormatQueryParam        -> priority 20 (checks ?format= param)
Module: FormatContentNegotiate  -> priority 30 (parses Accept header)
Module: FormatDefault           -> priority 40 (returns kernel default)
```

Each module can be independently enabled/disabled via config:

```toml
[modules]
format.route_override = true
format.query_param = true
format.content_negotiate = true
format.default = true
```

### Resolution Flow

1. Hook bus executes `response.format.resolve` handlers in priority order
2. First handler to return a format string + resolved=true wins
3. The resolved format string maps to a registered Formatter

## Formatter Port

```go
type Formatter interface {
    Name() string                              // e.g., "json", "html"
    ContentType() string                       // e.g., "application/json"
    Format(ctx *Context, resp *Response) ([]byte, error)
}
```

Formatters register with the kernel as named resources:

```go
kernel.Provide("formatter.json", jsonFormatter)
kernel.Provide("formatter.html", htmlFormatter)
```

## Built-in Formatters

### JSON Formatter
- Serializes `Data` via `encoding/json`
- Sets `Content-Type: application/json`
- Supports pretty-printing via config or query param (`?pretty=true`)

### HTML Formatter
- Uses Go's `html/template` by default
- Reads `Response.Template` to select the template
- `Data` is passed as the template context
- Template directory configurable via module config

## Response Pipeline

After format resolution, the response pipeline:

1. Fire `response.before_write` hooks (modules can modify response)
2. Look up the Formatter by resolved format name
3. Call `Formatter.Format()` to serialize
4. Set response headers (Content-Type, status code, custom headers)
5. Write bytes to `http.ResponseWriter`
6. Fire `response.after_write` hooks (logging, metrics)

## Error Handling

If `Response.Error` is non-nil:
1. Fire `request.error` hook (modules can intercept/transform)
2. If no module handles it, serialize a default error response in the resolved format
3. Default error response includes status code and message (no stack traces in production)
