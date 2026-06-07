---
title: Add middleware to a route group
weight: 10
---

Use this guide when you want a piece of cross-cutting logic (auth, rate
limiting, request logging) to wrap a set of routes.

## Solution

Register the middleware by name on the routing registry, then list it on a
group or route in TOML.

```go
import (
    "github.com/frob/nullspace/core/request"
    "github.com/frob/nullspace/core/routing"
    "github.com/frob/nullspace/kernel"
)

func (m *Module) Init(k *kernel.Kernel) error {
    reg, err := kernel.GetResource[*routing.Registry](k, "routing.registry")
    if err != nil {
        return err
    }
    reg.Middleware("auth", m.authMiddleware)
    return nil
}

func (m *Module) authMiddleware(next request.HandlerFunc) request.HandlerFunc {
    return func(ctx *request.Context) error {
        if !m.authorised(ctx) {
            ctx.Writer.WriteHeader(http.StatusUnauthorized)
            return nil
        }
        ctx.SetState("user", "alice")
        return next(ctx)
    }
}
```

Apply it to every route in a group:

```toml
[routing.groups.admin]
prefix     = "/api/admin"
format     = "json"
middleware = ["auth"]

[[routing.routes]]
group   = "admin"
path    = "/users"
handler = "users.list"

[[routing.routes]]
group   = "admin"
path    = "/users/:id"
handler = "users.get"
```

## Variations

### Add middleware to a single route

A route's `middleware` list is appended to its group's:

```toml
[[routing.routes]]
group      = "api"
path       = "/admin/posts"
handler    = "data.list"
collection = "posts"
middleware = ["auth"]
```

### Apply middleware globally

Register middleware directly on the request adapter to wrap every request:

```go
adapter, _ := kernel.GetResource[*request.Adapter](k, "request.adapter")
adapter.Use(m.requestLogger)
```

```go
func requestLogger(next request.HandlerFunc) request.HandlerFunc {
    return func(ctx *request.Context) error {
        start := time.Now()
        err := next(ctx)
        ctx.Logger().Info("request handled", "duration", time.Since(start))
        return err
    }
}
```

### Per-route in Go

```go
adapter.Router().Post("/admin/users", handler,
    request.WithRouteMiddleware(adminOnly),
)
```

## Ordering

Middleware runs in the order it appears in the list — first entry is
outermost. Group middleware runs before route middleware.

```toml
middleware = ["request_id", "auth", "audit"]
# request_id wraps auth wraps audit wraps handler
```

## See also

- [Register a custom handler]({{< relref "custom-handler" >}})
- [Hook into the request lifecycle]({{< relref "hooks" >}})
