---
title: Formats and resolvers
weight: 8
---

The response pipeline serializes handler output by:

1. Firing the `response.format.resolve` resolve hook.
2. Selecting the registered formatter by name.
3. Calling `Format` (batch) or the `StreamFormatter` methods (streaming).

## Built-in formatters

Registered by `core/response.Pipeline.Init`.

| Name   | Content-Type                  | Streaming                  | Notes                                                                  |
| ------ | ----------------------------- | -------------------------- | ---------------------------------------------------------------------- |
| `json` | `application/json`            | `application/x-ndjson` (NDJSON) | `?pretty=true` query param emits indented JSON.                  |
| `html` | `text/html; charset=utf-8`    | —                          | Requires `Response.Template`. Templates loaded from `[response] template_dir`. |
| `text` | `text/plain; charset=utf-8`   | —                          | Renders maps/structs as `Key: Value` lines; `body`/`content`/`message` fields become body text. |
| `ansi` | `text/plain; charset=utf-8`   | —                          | Same structure as `text` with ANSI color escapes (errors red, list indices cyan, status codes coloured by range). |

Only the `json` formatter implements `StreamFormatter`. When a stream
response is sent and the resolved format is not `json`, the pipeline
drains the iterator and falls back to batch serialization.

## Format resolvers

Each resolver is a module that registers a `response.format.resolve`
resolution hook. The first resolver (in priority order) to return
`resolved = true` wins. If none resolves, the pipeline uses
`[response] default_format`.

| Module name              | Priority | Source                                              | Module key                  |
| ------------------------ | -------- | --------------------------------------------------- | --------------------------- |
| `format.route_override`  | `10`     | Route metadata key `format` (set via `route.format` or group `format`). | `format.route_override`     |
| `format.query_param`     | `20`     | `?format=...` query parameter.                      | `format.query_param`        |
| `format.content_negotiate` | `30`   | `Accept` request header. Matches `application/json` → `json`, `text/html` → `html`, `text/plain` → `text`. `*/*` falls through. | `format.content_negotiate` |
| `format.default`         | `40`     | `[response] default_format`. Always resolves.       | `format.default`            |

Disable a resolver in `[modules]`:

```toml
[modules]
"format.query_param" = false
```

## Streaming

The pipeline switches to streaming mode when the request context has
`response.stream = true`. The adapter sets that flag when either of these
is true:

- The query string contains `stream=true`.
- The matched route metadata `stream` equals `"true"`.

Streaming uses `Pipeline.WriteStream(ctx, w, *StreamResponse)`. The
`StreamResponse.Meta` map is passed to `WriteStreamHeader`. The
`StreamResponse.Iter` provides items one at a time via
`Next() (any, error)`; iteration ends when `Next` returns `(nil, nil)`.

## Hook contract

```
response.format.resolve  (resolve)  -> (string, bool, error)
```

Custom resolvers register at any priority and return either:

- `("json", true, nil)` to set the format and short-circuit.
- `(nil, false, nil)` to defer to lower-priority resolvers.
- `(_, _, err)` to fail format resolution.
