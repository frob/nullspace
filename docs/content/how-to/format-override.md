---
title: Force a response format on a route
weight: 8
---

Use this guide when a route should always return a specific format, ignoring
the client's `Accept` header.

## Solution

Set the route's `format` field in TOML. The `format.route_override` resolver
runs first (priority 10) and short-circuits the rest of the chain.

```toml
[[routing.routes]]
path    = "/api/health"
handler = "health.check"
format  = "json"
```

`/api/health` now returns JSON regardless of `Accept`, `?format=`, or the
configured default.

### From a group

Groups apply `format` to every route inside them. A route's own `format`
overrides the group.

```toml
[routing.groups.api]
prefix = "/api"
format = "json"

[[routing.routes]]
group   = "api"
path    = "/posts"
handler = "data.list"
collection = "posts"
```

### From a Go route

```go
adapter.Router().Get("/api/posts", listPosts,
    request.WithMeta("format", "json"),
)
```

## Format resolution chain

| Priority | Resolver                   | Source                       |
| -------- | -------------------------- | ---------------------------- |
| 10       | `format.route_override`    | Route metadata               |
| 20       | `format.query_param`       | `?format=json`               |
| 30       | `format.content_negotiate` | `Accept` header              |
| 40       | `format.default`           | `[response] default_format`  |

The first resolver to return a format wins. Setting `format` on a route or
group locks the response into that format.

## Variations

### Let the client pick (the default)

Omit `format` and the chain falls through to query parameter, content
negotiation, and finally the configured default.

```toml
[response]
default_format = "json"
```

### Disable individual resolvers

If you want to ignore the query parameter entirely, disable that resolver:

```toml
[modules]
"format.query_param" = false
```

## See also

- [Add a custom formatter]({{< relref "custom-formatter" >}})
- [Stream large lists as NDJSON]({{< relref "streaming" >}})
