---
title: Using the library
weight: 2
---

When you need custom routes, custom modules, or extra transports, import
Nullspace as a Go library.

## Install

```bash
mkdir myapp && cd myapp
go mod init myapp
go get github.com/frob/nullspace
```

## A minimal `main.go`

```go
package main

import (
    "context"
    "net/http"
    "os"
    "os/signal"
    "syscall"

    "github.com/frob/nullspace/core/nslog"
    "github.com/frob/nullspace/core/request"
    "github.com/frob/nullspace/core/response"
    "github.com/frob/nullspace/kernel"
)

func main() {
    k := kernel.New()

    adapter := request.NewAdapter()
    pipeline := response.NewPipeline()

    k.Use(nslog.New())
    k.Use(adapter)
    k.Use(pipeline)
    k.Use(response.NewFormatDefault())

    ctx := context.Background()
    if err := k.Init(ctx); err != nil {
        panic(err)
    }

    adapter.Router().Get("/hello", func(ctx *request.Context) error {
        resp := response.NewResponse(http.StatusOK, map[string]string{
            "message": "hello world",
        })
        return pipeline.Write(ctx.Context(), ctx.Writer, resp)
    })

    if err := k.Start(ctx); err != nil {
        panic(err)
    }

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    k.Stop(ctx)
}
```

Run it:

```bash
go run .
```

```bash
curl http://localhost:8080/hello
# {"message":"hello world"}
```

## What the registration order means

```
nslog       → logging available to every later module
adapter     → HTTP listener + router on the service locator
pipeline    → response formatter chain
format.*    → resolvers (route → query → negotiate → default)
data.*      → static, file, sql (only what you need)
routing     → loads TOML routes
websocket   → registers WS upgrade handler
your apps   → register handlers, hooks, custom modules
```

Modules registered later can read resources registered by earlier modules
through the service locator (`kernel.GetResource[T](k, "key")`).

## Next

- [Wire Nullspace as a library]({{< relref "/tutorials/library" >}}) —
  full walkthrough, including writing your own module.
- [Reference: service locator keys]({{< relref "/reference/resources" >}}).
- [Examples]({{< relref "/examples" >}}) — every example program is a
  library-form project.
