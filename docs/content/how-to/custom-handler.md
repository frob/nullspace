---
title: Register a custom handler
weight: 2
---

Use this guide when a TOML route needs application logic that the built-in
handlers can't provide.

## Solution

Register a handler by name on the routing registry during your module's
`Init`. Then reference the same name from a TOML route.

```go
package myapp

import (
    "net/http"

    "github.com/frob/nullspace/core/request"
    "github.com/frob/nullspace/core/response"
    "github.com/frob/nullspace/core/routing"
    "github.com/frob/nullspace/kernel"
)

type Module struct{}

func New() *Module                                   { return &Module{} }
func (m *Module) Name() string                       { return "myapp" }
func (m *Module) Start(ctx context.Context) error    { return nil }
func (m *Module) Stop(ctx context.Context) error     { return nil }

func (m *Module) Init(k *kernel.Kernel) error {
    reg, err := kernel.GetResource[*routing.Registry](k, "routing.registry")
    if err != nil {
        return err
    }
    pipeline, err := kernel.GetResource[*response.Pipeline](k, "response.pipeline")
    if err != nil {
        return err
    }

    reg.HandleFunc("health.check", func(ctx *request.Context) error {
        resp := response.NewResponse(http.StatusOK, map[string]string{
            "status": "ok",
        })
        return pipeline.Write(ctx.Context(), ctx.Writer, resp)
    })

    return nil
}
```

Register the module in `main.go`, after the routing module:

```go
k.Use(routing.New())
k.Use(myapp.New())
```

Reference the handler from TOML:

```toml
[[routing.routes]]
group   = "api"
path    = "/health"
handler = "health.check"
```

## Variations

### Read a path parameter

```go
reg.HandleFunc("post.show", func(ctx *request.Context) error {
    id := ctx.Param("id")
    // ...
})
```

```toml
[[routing.routes]]
path    = "/posts/:id"
handler = "post.show"
```

### Receive pre-loaded entity data

When a TOML route sets a `collection` and points at a custom handler, the
routing module pre-loads the entity (or list) and stores it in the context
state bag.

```toml
[[routing.routes]]
path       = "/custom/:id"
handler    = "post.render"
collection = "posts"
```

```go
reg.HandleFunc("post.render", func(ctx *request.Context) error {
    entity, _ := ctx.State("data.entity") // single entity (parameterised path)
    // List handlers receive "data.entities" instead.
    // ...
})
```

### Go routes alongside TOML

You can register routes programmatically — they merge with TOML routes:

```go
adapter, _ := kernel.GetResource[*request.Adapter](k, "request.adapter")
adapter.Router().Get("/api/posts/:id", getPost,
    request.WithMeta("format", "json"),
)
```

## See also

- [Define routes in TOML]({{< relref "routes-toml" >}})
- [Add middleware to a route group]({{< relref "middleware" >}})
